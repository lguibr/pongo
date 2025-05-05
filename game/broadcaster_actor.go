// File: game/broadcaster_actor.go
// File: backend/game/broadcaster_actor.go
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
			if a.selfPID != nil { pidStr = a.selfPID.String() }
			fmt.Printf("PANIC recovered in BroadcasterActor %s Receive: %v\nStack trace:\n%s\n", pidStr, r, string(debug.Stack()))
		}
	}()

	if a.selfPID == nil { a.selfPID = ctx.Self() }

	switch msg := ctx.Message().(type) {
	case bollywood.Started:
		// Actor started
	case AddClient:
		if msg.Conn != nil { a.mu.Lock(); a.clients[msg.Conn] = true; a.mu.Unlock() }
	case RemoveClient:
		if msg.Conn != nil {
			a.mu.Lock()
			if _, exists := a.clients[msg.Conn]; exists { delete(a.clients, msg.Conn) }
			a.mu.Unlock()
		}
	case BroadcastUpdatesCommand:
		a.broadcastUpdates(ctx, msg.Updates)
	case GameOverMessage:
		fmt.Printf("Broadcaster %s: Received GameOverMessage for room %s. Broadcasting and closing connections.\n", a.selfPID, msg.RoomPID)
		a.broadcastGameOverAndClose(ctx, msg)
	case bollywood.Stopping:
		// fmt.Printf("Broadcaster %s: Stopping. Closing remaining connections.\n", a.selfPID) // Removed log
		a.closeAllConnections(ctx)
	case bollywood.Stopped:
		// Actor stopped
	default:
		// fmt.Printf("BroadcasterActor %s: Received unknown message type: %T\n", a.selfPID, msg) // Removed log
	}
}

// broadcastUpdates sends a batch of game updates to all registered clients using JSON.Send.
func (a *BroadcasterActor) broadcastUpdates(ctx bollywood.Context, updates []interface{}) {
	if len(updates) == 0 { return }

	a.mu.RLock()
	clientsToSend := make([]*websocket.Conn, 0, len(a.clients))
	for conn := range a.clients { clientsToSend = append(clientsToSend, conn) }
	a.mu.RUnlock()

	if len(clientsToSend) == 0 { return }

	batchMsg := GameUpdatesBatch{ MessageType: "gameUpdates", Updates: updates }

	disconnectedClients := []*websocket.Conn{}
	for _, ws := range clientsToSend {
		errSend := websocket.JSON.Send(ws, batchMsg)
		if errSend != nil {
			errStr := errSend.Error()
			isClosedErr := strings.Contains(errStr, "use of closed network connection") ||
				strings.Contains(errStr, "broken pipe") ||
				strings.Contains(errStr, "connection reset by peer") ||
				strings.Contains(errStr, "EOF") ||
				strings.Contains(errStr, "write: connection timed out")
			isBufferErr := strings.Contains(errStr, "no buffer space available")

			if isClosedErr || isBufferErr {
				disconnectedClients = append(disconnectedClients, ws)
				if isBufferErr { fmt.Printf("WARN: BroadcasterActor %s: Buffer space error for client %s. Marking for disconnect.\n", a.selfPID, ws.RemoteAddr()) }
			} else {
				fmt.Printf("ERROR: BroadcasterActor %s: Failed to write update batch to client %s: %v\n", a.selfPID, ws.RemoteAddr(), errSend)
			}
		}
	}

	if len(disconnectedClients) > 0 { a.handleDisconnects(ctx, disconnectedClients) }
}

// broadcastGameOverAndClose sends the GameOverMessage using JSON.Send and then closes all connections.
func (a *BroadcasterActor) broadcastGameOverAndClose(ctx bollywood.Context, msg GameOverMessage) {
	a.mu.RLock()
	clientsToSend := make([]*websocket.Conn, 0, len(a.clients))
	for conn := range a.clients { clientsToSend = append(clientsToSend, conn) }
	a.mu.RUnlock()

	if len(clientsToSend) == 0 { return }

	for _, ws := range clientsToSend {
		errSend := websocket.JSON.Send(ws, msg)
		if errSend != nil { fmt.Printf("WARN: BroadcasterActor %s: Failed to send GameOverMessage to client %s: %v\n", a.selfPID, ws.RemoteAddr(), errSend) }
	}
	a.closeAllConnections(ctx)
}

// closeAllConnections closes all managed WebSocket connections and notifies GameActor.
func (a *BroadcasterActor) closeAllConnections(ctx bollywood.Context) {
	a.mu.Lock()
	clientsToClose := make([]*websocket.Conn, 0, len(a.clients))
	for conn := range a.clients { clientsToClose = append(clientsToClose, conn) }
	a.clients = make(map[*websocket.Conn]bool) // Clear the map correctly
	a.mu.Unlock()

	if len(clientsToClose) > 0 {
		// fmt.Printf("Broadcaster %s: Closing %d connections.\n", a.selfPID, len(clientsToClose)) // Removed log
		for _, ws := range clientsToClose {
			_ = ws.Close()
			if a.gameActorPID != nil && ctx.Engine() != nil { ctx.Engine().Send(a.gameActorPID, PlayerDisconnect{WsConn: ws}, a.selfPID) }
		}
	}
}

// handleDisconnects removes clients and notifies GameActor.
func (a *BroadcasterActor) handleDisconnects(ctx bollywood.Context, disconnectedClients []*websocket.Conn) {
	a.mu.Lock()
	for _, ws := range disconnectedClients {
		if _, exists := a.clients[ws]; exists { delete(a.clients, ws) }
	}
	a.mu.Unlock()

	if a.gameActorPID != nil && ctx.Engine() != nil {
		for _, ws := range disconnectedClients { ctx.Engine().Send(a.gameActorPID, PlayerDisconnect{WsConn: ws}, a.selfPID) }
	}
}