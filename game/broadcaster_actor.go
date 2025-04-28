// File: game/broadcaster_actor.go
package game

import (
	"fmt"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/lguibr/bollywood"
	"golang.org/x/net/websocket"
)

// BroadcasterActor manages sending game state updates to clients in a room.
type BroadcasterActor struct {
	clients      map[*websocket.Conn]bool // Set of active connections
	mu           sync.RWMutex             // Protects the clients map
	selfPID      *bollywood.PID
	gameActorPID *bollywood.PID // PID of the GameActor to notify on disconnect
}

// NewBroadcasterProducer creates a producer for BroadcasterActor.
func NewBroadcasterProducer(gameActorPID *bollywood.PID) bollywood.Producer {
	return func() bollywood.Actor {
		return &BroadcasterActor{
			clients:      make(map[*websocket.Conn]bool),
			gameActorPID: gameActorPID,
		}
	}
}

// Receive handles messages for the BroadcasterActor.
func (a *BroadcasterActor) Receive(ctx bollywood.Context) {
	defer func() {
		if r := recover(); r != nil {
			pidStr := "unknown"
			if a.selfPID != nil {
				pidStr = a.selfPID.String()
			}
			fmt.Printf("PANIC recovered in BroadcasterActor %s Receive: %v\nStack trace:\n%s\n", pidStr, r, string(debug.Stack()))
		}
	}()

	if a.selfPID == nil {
		a.selfPID = ctx.Self()
	}

	switch msg := ctx.Message().(type) {
	case bollywood.Started:
		// Actor started

	case AddClient:
		if msg.Conn != nil {
			a.mu.Lock()
			a.clients[msg.Conn] = true
			a.mu.Unlock()
		}

	case RemoveClient:
		if msg.Conn != nil {
			a.mu.Lock()
			_, exists := a.clients[msg.Conn]
			if exists {
				delete(a.clients, msg.Conn)
			}
			a.mu.Unlock()
		}

	case BroadcastStateCommand:
		// Send the GameState struct using websocket.JSON.Send
		a.broadcast(ctx, msg.State) // Pass the GameState struct

	case bollywood.Stopping:
		// Actor stopping

	case bollywood.Stopped:
		// Actor stopped

	default:
		fmt.Printf("BroadcasterActor %s: Received unknown message type: %T\n", a.selfPID, msg)
	}
}

// broadcast sends the GameState struct to all registered clients using JSON encoding.
func (a *BroadcasterActor) broadcast(ctx bollywood.Context, state GameState) {
	// Check if the state is valid (basic check, could be more thorough)
	if state.Canvas == nil {
		fmt.Printf("WARN: BroadcasterActor %s received GameState with nil Canvas to broadcast.\n", a.selfPID)
		// Decide if you want to return or send a minimal state
		// return
	}

	a.mu.RLock()
	clientsToSend := make([]*websocket.Conn, 0, len(a.clients))
	for conn := range a.clients {
		clientsToSend = append(clientsToSend, conn)
	}
	a.mu.RUnlock()

	if len(clientsToSend) == 0 {
		return
	}

	disconnectedClients := []*websocket.Conn{}

	for _, ws := range clientsToSend {
		// Use websocket.JSON.Send for sending the struct as JSON text message
		err := websocket.JSON.Send(ws, &state) // Send pointer to the state struct
		if err != nil {
			// Check common closed connection errors
			errStr := err.Error()
			isClosedErr := strings.Contains(errStr, "use of closed network connection") ||
				strings.Contains(errStr, "broken pipe") ||
				strings.Contains(errStr, "connection reset by peer") ||
				strings.Contains(errStr, "EOF") ||
				strings.Contains(errStr, "write: connection timed out")

			if isClosedErr {
				disconnectedClients = append(disconnectedClients, ws)
			} else {
				// Log other unexpected errors
				fmt.Printf("ERROR: BroadcasterActor %s: Failed to write state to client %s: %v\n", a.selfPID, ws.RemoteAddr(), err)
			}
		}
	}

	// Notify self and GameActor about disconnected clients
	if len(disconnectedClients) > 0 {
		for _, ws := range disconnectedClients {
			// Tell self to remove the client
			ctx.Engine().Send(a.selfPID, RemoveClient{Conn: ws}, nil)
			// Tell GameActor the connection dropped
			if a.gameActorPID != nil {
				ctx.Engine().Send(a.gameActorPID, PlayerDisconnect{WsConn: ws}, a.selfPID)
			}
		}
	}
}
