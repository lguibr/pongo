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
				// Optionally close connection here too, though GameActor/Handler usually does
				// _ = msg.Conn.Close()
			}
			a.mu.Unlock()
		}

	case BroadcastStateCommand:
		// Send the GameState struct using websocket.JSON.Send
		a.broadcastState(ctx, msg.State) // Pass the GameState struct

	case GameOverMessage:
		fmt.Printf("Broadcaster %s: Received GameOverMessage for room %s. Broadcasting and closing connections.\n", a.selfPID, msg.RoomPID)
		a.broadcastGameOverAndClose(ctx, msg)

	case bollywood.Stopping:
		// Actor stopping - maybe close remaining connections?
		fmt.Printf("Broadcaster %s: Stopping. Closing remaining connections.\n", a.selfPID)
		a.closeAllConnections(ctx)

	case bollywood.Stopped:
		// Actor stopped

	default:
		fmt.Printf("BroadcasterActor %s: Received unknown message type: %T\n", a.selfPID, msg)
	}
}

// broadcastState sends the GameState struct to all registered clients using JSON encoding.
func (a *BroadcasterActor) broadcastState(ctx bollywood.Context, state GameState) {
	if state.Canvas == nil {
		// fmt.Printf("WARN: BroadcasterActor %s received GameState with nil Canvas to broadcast.\n", a.selfPID) // Reduce noise
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
		err := websocket.JSON.Send(ws, &state)
		if err != nil {
			errStr := err.Error()
			isClosedErr := strings.Contains(errStr, "use of closed network connection") ||
				strings.Contains(errStr, "broken pipe") ||
				strings.Contains(errStr, "connection reset by peer") ||
				strings.Contains(errStr, "EOF") ||
				strings.Contains(errStr, "write: connection timed out")

			if isClosedErr {
				disconnectedClients = append(disconnectedClients, ws)
			} else {
				fmt.Printf("ERROR: BroadcasterActor %s: Failed to write state to client %s: %v\n", a.selfPID, ws.RemoteAddr(), err)
			}
		}
	}

	// Notify self and GameActor about disconnected clients detected during broadcast
	if len(disconnectedClients) > 0 {
		a.handleDisconnects(ctx, disconnectedClients)
	}
}

// broadcastGameOverAndClose sends the GameOverMessage and then closes all connections.
func (a *BroadcasterActor) broadcastGameOverAndClose(ctx bollywood.Context, msg GameOverMessage) {
	a.mu.RLock()
	clientsToSend := make([]*websocket.Conn, 0, len(a.clients))
	for conn := range a.clients {
		clientsToSend = append(clientsToSend, conn)
	}
	a.mu.RUnlock()

	if len(clientsToSend) == 0 {
		return
	}

	// Send the game over message first
	for _, ws := range clientsToSend {
		err := websocket.JSON.Send(ws, &msg)
		if err != nil {
			// Log error but proceed to close anyway
			fmt.Printf("WARN: BroadcasterActor %s: Failed to send GameOverMessage to client %s: %v\n", a.selfPID, ws.RemoteAddr(), err)
		}
	}

	// Now close all connections managed by this broadcaster
	a.closeAllConnections(ctx)
}

// closeAllConnections closes all managed WebSocket connections and notifies GameActor.
func (a *BroadcasterActor) closeAllConnections(ctx bollywood.Context) {
	a.mu.Lock() // Full lock to safely iterate and modify
	clientsToClose := make([]*websocket.Conn, 0, len(a.clients))
	for conn := range a.clients {
		clientsToClose = append(clientsToClose, conn)
	}
	// Clear the map immediately after copying
	a.clients = make(map[*websocket.Conn]bool)
	a.mu.Unlock()

	if len(clientsToClose) > 0 {
		fmt.Printf("Broadcaster %s: Closing %d connections.\n", a.selfPID, len(clientsToClose))
		for _, ws := range clientsToClose {
			_ = ws.Close() // Attempt to close
			// Notify GameActor about the disconnection
			if a.gameActorPID != nil && ctx.Engine() != nil {
				ctx.Engine().Send(a.gameActorPID, PlayerDisconnect{WsConn: ws}, a.selfPID)
			}
		}
	}
}

// handleDisconnects removes clients and notifies GameActor.
func (a *BroadcasterActor) handleDisconnects(ctx bollywood.Context, disconnectedClients []*websocket.Conn) {
	a.mu.Lock()
	for _, ws := range disconnectedClients {
		delete(a.clients, ws)
	}
	a.mu.Unlock()

	// Notify GameActor about the disconnections
	if a.gameActorPID != nil && ctx.Engine() != nil {
		for _, ws := range disconnectedClients {
			ctx.Engine().Send(a.gameActorPID, PlayerDisconnect{WsConn: ws}, a.selfPID)
		}
	}
}
