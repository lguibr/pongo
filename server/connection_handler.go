
package server

import (
	"encoding/json"
	"errors" // Import errors
	"fmt"
	"net"     // Import net package
	"reflect" // Import reflect
	"runtime/debug"
	"sync" // Import sync
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/game"
	"golang.org/x/net/websocket"
)

// errActorStopping is a specific error used to signal cleanup due to actor stopping.
var errActorStopping = errors.New("connection handler actor stopping")

// ConnectionHandlerActor manages a single WebSocket connection lifecycle.
type ConnectionHandlerActor struct {
	conn           *websocket.Conn
	engine         *bollywood.Engine
	roomManagerPID *bollywood.PID
	gameActorPID   *bollywood.PID // PID of the assigned GameActor
	selfPID        *bollywood.PID
	connAddr       string
	stopReadLoop   chan struct{} // Channel to signal readLoop to stop
	readLoopExited chan struct{} // Channel to signal readLoop has exited
	done           chan struct{} // Channel to signal handler completion
	isAssigned     bool          // Flag to track if assigned to a GameActor
	closeOnce      sync.Once     // Ensures done channel is closed only once
}

// ConnectionHandlerArgs holds arguments for creating the actor.
type ConnectionHandlerArgs struct {
	Conn           *websocket.Conn
	Engine         *bollywood.Engine
	RoomManagerPID *bollywood.PID
	Done           chan struct{} // Add done channel
}

// NewConnectionHandlerProducer creates a producer for ConnectionHandlerActor.
func NewConnectionHandlerProducer(args ConnectionHandlerArgs) bollywood.Producer {
	return func() bollywood.Actor {
		addr := "unknown"
		if args.Conn != nil {
			addr = args.Conn.RemoteAddr().String()
		}
		return &ConnectionHandlerActor{
			conn:           args.Conn,
			engine:         args.Engine,
			roomManagerPID: args.RoomManagerPID,
			connAddr:       addr,
			stopReadLoop:   make(chan struct{}),
			readLoopExited: make(chan struct{}),
			done:           args.Done, // Store done channel
			isAssigned:     false,     // Initialize as not assigned
			// closeOnce is initialized automatically
		}
	}
}

// Receive handles messages for the ConnectionHandlerActor.
func (a *ConnectionHandlerActor) Receive(ctx bollywood.Context) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC recovered in ConnectionHandlerActor %s Receive: %v\nStack trace:\n%s\n", a.connAddr, r, string(debug.Stack()))
			a.cleanup(ctx, fmt.Errorf("panic in Receive: %v", r))
		}
	}()

	if a.selfPID == nil {
		a.selfPID = ctx.Self()
	}

	switch msg := ctx.Message().(type) {
	case bollywood.Started:
		if a.roomManagerPID != nil {
			// Use engine.Send directly here as well for consistency
			a.engine.Send(a.roomManagerPID, game.FindRoomRequest{ReplyTo: a.selfPID}, nil)
		} else {
			fmt.Printf("ERROR: ConnectionHandlerActor %s: No RoomManagerPID. Stopping.\n", a.connAddr)
			a.cleanup(ctx, fmt.Errorf("missing RoomManagerPID"))
		}

	case game.AssignRoomResponse:
		if msg.RoomPID == nil {
			fmt.Printf("ConnectionHandlerActor %s: Received nil RoomPID assignment. Closing connection.\n", a.connAddr)
			a.cleanup(ctx, fmt.Errorf("room assignment failed (nil PID)"))
			return
		}
		a.gameActorPID = msg.RoomPID
		a.isAssigned = true // Mark as assigned *before* starting readLoop
		// Use engine.Send
		a.engine.Send(a.gameActorPID, game.AssignPlayerToRoom{WsConn: a.conn}, a.selfPID)
		// Pass engine and selfPID explicitly to readLoop
		go a.readLoop(a.engine, a.selfPID) // Start readLoop *after* assignment is processed

	case game.InternalReadLoopMsg:
		// Removed log: fmt.Printf("ConnectionHandlerActor %s: Received InternalReadLoopMsg\n", a.connAddr)
		if a.isAssigned && a.gameActorPID != nil { // Check if assigned before forwarding
			// Use engine.Send
			a.engine.Send(a.gameActorPID, game.ForwardedPaddleDirection{
				WsConn:    a.conn,
				Direction: msg.Payload,
			}, a.selfPID)
		} else {
			// fmt.Printf("WARN: ConnectionHandlerActor %s received input before game assignment. Dropping.\n", a.connAddr) // Removed log
		}

	case *net.OpError:
		// fmt.Printf("ConnectionHandlerActor %s: Received *net.OpError: %v. Cleaning up.\n", a.connAddr, msg) // Removed log
		a.cleanup(ctx, msg)

	case error:
		// Check if it's the specific "read loop exited" error to avoid redundant cleanup logs
		if msg.Error() != "read loop exited" {
			// fmt.Printf("ConnectionHandlerActor %s: Received error: %v. Cleaning up.\n", a.connAddr, msg) // Removed log
		} else {
			// fmt.Printf("ConnectionHandlerActor %s: Received notification: %v. Cleaning up.\n", a.connAddr, msg) // Reduce noise
		}
		a.cleanup(ctx, msg)

	case bollywood.Stopping:
		// fmt.Printf("ConnectionHandlerActor %s: Received Stopping message.\n", a.connAddr) // Reduce noise
		a.signalAndWaitForReadLoop()
		a.performCleanupActions(ctx, errActorStopping) // Pass specific error

	case bollywood.Stopped:
		// fmt.Printf("ConnectionHandlerActor %s: Received Stopped message.\n", a.connAddr) // Reduce noise
		// Use sync.Once to close done channel
		a.closeOnce.Do(func() {
			if a.done != nil {
				// fmt.Printf("ConnectionHandlerActor %s: Closing done channel.\n", a.connAddr) // Reduce noise
				close(a.done)
				a.done = nil // Prevent future attempts if somehow accessed again
			}
		})

	default:
		// fmt.Printf("ConnectionHandlerActor %s: Received unexpected message type in Receive: %T, Value: %+v\n", a.connAddr, msg, msg) // Removed log
		if val := reflect.ValueOf(msg); val.Kind() == reflect.Ptr {
			// fmt.Printf("ConnectionHandlerActor %s: Underlying type: %T\n", a.connAddr, reflect.Indirect(val).Interface()) // Removed log
		}
	}
}

// readLoop handles reading messages from the WebSocket connection.
// Takes engine and selfPID as arguments.
func (a *ConnectionHandlerActor) readLoop(engine *bollywood.Engine, selfPID *bollywood.PID) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC recovered in ConnectionHandlerActor %s readLoop: %v\nStack trace:\n%s\n", a.connAddr, r, string(debug.Stack()))
		}
		close(a.readLoopExited)
		// fmt.Printf("ConnectionHandlerActor %s: Read loop finished.\n", a.connAddr) // Reduce noise
		// Send error back to the actor instance using the provided engine/PID
		if engine != nil && selfPID != nil {
			readLoopExitErr := errors.New("read loop exited")
			engine.Send(selfPID, readLoopExitErr, nil) // Use captured engine/pid
		}
	}()

	// fmt.Printf("ConnectionHandlerActor %s: Read loop started.\n", a.connAddr) // Reduce noise
	for {
		select {
		case <-a.stopReadLoop:
			// fmt.Printf("ConnectionHandlerActor %s: Read loop received stop signal.\n", a.connAddr) // Reduce noise
			return
		default:
		}

		var message json.RawMessage
		readTimeout := 90 * time.Second
		if a.conn == nil {
			// fmt.Printf("ConnectionHandlerActor %s: Connection is nil in readLoop, exiting.\n", a.connAddr) // Reduce noise
			return
		}
		_ = a.conn.SetReadDeadline(time.Now().Add(readTimeout))
		err := websocket.JSON.Receive(a.conn, &message)
		if a.conn != nil {
			_ = a.conn.SetReadDeadline(time.Time{})
		}

		if err != nil {
			select {
			case <-a.stopReadLoop:
				// fmt.Printf("ConnectionHandlerActor %s: Read loop exiting due to stop signal after read error (%v).\n", a.connAddr, err) // Reduce noise
			default:
				// fmt.Printf("ConnectionHandlerActor %s: Read error: %v. Exiting read loop.\n", a.connAddr, err) // Reduce noise
			}
			return // Exit loop on error
		}

		// Send message back to the actor instance using the provided engine/PID
		if selfPID != nil && engine != nil {
			// Use engine.Send directly, no need for mailbox check here
			engine.Send(selfPID, game.InternalReadLoopMsg{Payload: []byte(message)}, nil)
		} else {
			fmt.Printf("ERROR: ConnectionHandlerActor %s: Cannot send read message, engine or selfPID is nil in readLoop.\n", a.connAddr)
		}
	}
}

// signalAndWaitForReadLoop tells the readLoop goroutine to exit and waits for confirmation.
func (a *ConnectionHandlerActor) signalAndWaitForReadLoop() {
	select {
	case <-a.stopReadLoop:
		// Already closed or closing
		return
	default:
		// Close the channel to signal stop
		close(a.stopReadLoop)
	}

	// Close the connection to potentially unblock the readLoop's Receive call
	if a.conn != nil {
		_ = a.conn.Close()
	}

	// Wait for the readLoop to signal it has exited
	select {
	case <-a.readLoopExited:
		// fmt.Printf("ConnectionHandlerActor %s: Read loop confirmed exited.\n", a.connAddr) // Reduce noise
	case <-time.After(2 * time.Second):
		fmt.Printf("WARN: ConnectionHandlerActor %s: Timeout waiting for read loop to exit.\n", a.connAddr)
	}
}

// cleanup is called when the connection terminates (readLoop exits) or the actor stops.
func (a *ConnectionHandlerActor) cleanup(ctx bollywood.Context, reason error) {
	// fmt.Printf("ConnectionHandlerActor %s: Initiating cleanup (Reason: %v).\n", a.connAddr, reason) // Reduce noise
	a.signalAndWaitForReadLoop()
	a.performCleanupActions(ctx, reason)
	// Only stop self if the cleanup wasn't triggered by the actor already stopping
	if !errors.Is(reason, errActorStopping) {
		if a.engine != nil && a.selfPID != nil {
			// fmt.Printf("ConnectionHandlerActor %s: Cleanup requesting self stop.\n", a.connAddr) // Reduce noise
			a.engine.Stop(a.selfPID)
		}
	} else {
		// fmt.Printf("ConnectionHandlerActor %s: Cleanup initiated by Stopping message, not stopping self again.\n", a.connAddr) // Reduce noise
	}
}

// performCleanupActions sends disconnect and nils the connection reference.
func (a *ConnectionHandlerActor) performCleanupActions(ctx bollywood.Context, reason error) {
	connToDisconnect := a.conn // Capture connection before potentially nil-ing it

	// Send PlayerDisconnect only if assigned, connection exists, and not already stopping
	if a.isAssigned && a.gameActorPID != nil && connToDisconnect != nil && !errors.Is(reason, errActorStopping) {
		// fmt.Printf("ConnectionHandlerActor %s: Sending PlayerDisconnect to %s.\n", a.connAddr, a.gameActorPID) // Reduce noise
		if a.engine != nil && a.selfPID != nil {
			a.engine.Send(a.gameActorPID, game.PlayerDisconnect{WsConn: connToDisconnect}, a.selfPID)
		}
	} else if a.gameActorPID != nil {
		// Log why disconnect wasn't sent
		// fmt.Printf("ConnectionHandlerActor %s: Not sending PlayerDisconnect to %s (Reason: %v, Assigned: %t, ConnNil: %t).\n",
		// 	a.connAddr, a.gameActorPID, reason, a.isAssigned, connToDisconnect == nil) // Reduce noise
	}

	// Close and nil the connection reference
	if a.conn != nil {
		_ = a.conn.Close()
		a.conn = nil
	}
	a.isAssigned = false // Mark as unassigned during cleanup
}