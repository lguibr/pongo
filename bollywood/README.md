
# Bollywood Actor Library

Bollywood is a lightweight Actor Model implementation for Go, inspired by the principles of asynchronous message passing and state encapsulation. It aims to provide a simple yet powerful way to build concurrent applications using actors, channels, and minimal locking.

## Core Concepts

*   **Engine:** The central coordinator responsible for spawning, managing, and terminating actors.
*   **Actor:** An entity that encapsulates state and behavior. It implements the `Actor` interface, primarily the `Receive(Context)` method.
*   **PID (Process ID):** An opaque identifier used to reference and send messages to a specific actor instance.
*   **Context:** Provided to an actor's `Receive` method, allowing it to interact with the system (e.g., get its PID, sender PID, spawn children, send messages).
*   **Props:** Configuration object used to spawn new actors, specifying how to create an actor instance.
*   **Message:** Any Go `interface{}` value sent between actors.

## Basic Usage

```go
package main

import (
	"fmt"
	"time"

	"github.com/lguibr/pongo/bollywood" // Adjust import path as needed
)

// Define an actor struct
type MyActor struct {
	count int
}

// Implement the Actor interface
func (a *MyActor) Receive(ctx bollywood.Context) {
	switch msg := ctx.Message().(type) {
	case string:
		fmt.Printf("Actor %s received string: %s\n", ctx.Self().ID, msg)
		a.count++
	case int:
		fmt.Printf("Actor %s received int: %d\n", ctx.Self().ID, msg)
		// Example: Respond to sender
		if ctx.Sender() != nil {
			ctx.Engine().Send(ctx.Sender(), fmt.Sprintf("Processed %d", msg), ctx.Self())
		}
	case bollywood.Started:
        fmt.Printf("Actor %s started\n", ctx.Self().ID)
    case bollywood.Stopping:
        fmt.Printf("Actor %s stopping\n", ctx.Self().ID)
    case bollywood.Stopped:
        fmt.Printf("Actor %s stopped\n", ctx.Self().ID)
	default:
		fmt.Printf("Actor %s received unknown message type\n", ctx.Self().ID)
	}
}

// Actor producer function
func newMyActor() bollywood.Actor {
	return &MyActor{}
}

func main() {
	// Create an actor engine
	engine := bollywood.NewEngine()

	// Define properties for the actor
	props := bollywood.NewProps(newMyActor)

	// Spawn the actor
	pid := engine.Spawn(props)

	// Send messages
	engine.Send(pid, "hello world", nil) // nil sender for messages from outside actors
	engine.Send(pid, 42, nil)

    // Spawn another actor to receive a response
    responderProps := bollywood.NewProps(func() bollywood.Actor {
        return &MyActor{} // Using MyActor for simplicity, could be a different type
    })
    responderPID := engine.Spawn(responderProps)

    // Send a message and expect a response
    engine.Send(pid, 100, responderPID)


	// Allow time for messages to be processed
	time.Sleep(100 * time.Millisecond)

	// Stop the actor
	engine.Stop(pid)
    engine.Stop(responderPID)

	// Allow time for stop messages
	time.Sleep(50 * time.Millisecond)

    fmt.Println("Engine finished")
}

```

## Related Modules

*   [PonGo Game](../game/README.md)
*   [Main Project](../README.md)