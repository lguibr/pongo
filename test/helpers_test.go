// File: test/helpers.go
package test

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

// ReadWsJSONMessage reads a JSON message from the WebSocket with a timeout.
// It handles setting/clearing read deadlines and checks for common errors.
// Renamed to be exported.
func ReadWsJSONMessage(t *testing.T, ws *websocket.Conn, timeout time.Duration, v interface{}) error {
	t.Helper()
	if ws == nil {
		return errors.New("websocket connection is nil")
	}

	readDone := make(chan error, 1)
	var readErr error

	go func() {
		// It's crucial to set deadline *before* Receive
		setReadErr := ws.SetReadDeadline(time.Now().Add(timeout))
		if setReadErr != nil {
			// Check if the error is due to closed connection, which might be expected
			if errors.Is(setReadErr, net.ErrClosed) || strings.Contains(setReadErr.Error(), "use of closed network connection") {
				readDone <- io.EOF // Signal EOF if connection already closed
				return
			}
			// Report other deadline errors
			readDone <- fmt.Errorf("failed to set read deadline: %w", setReadErr)
			return
		}

		// Attempt to receive JSON message
		err := websocket.JSON.Receive(ws, v)

		// Clear deadline immediately after Receive returns, regardless of error
		clearDeadlineErr := ws.SetReadDeadline(time.Time{})
		if clearDeadlineErr != nil && !errors.Is(clearDeadlineErr, net.ErrClosed) {
			// Log if clearing deadline fails unexpectedly (but prioritize Receive error)
			// Use t.Logf for logging within tests
			// t.Logf("Warning: Failed to clear read deadline: %v", clearDeadlineErr)
		}

		// Send the result of Receive (which could be nil, io.EOF, or other errors)
		readDone <- err
	}()

	// Wait for the read operation or overall timeout
	select {
	case readErr = <-readDone:
		return readErr // Return error from Receive (can be nil, io.EOF, etc.)
	case <-time.After(timeout + 500*time.Millisecond): // Slightly longer overall timeout
		// If the select times out, it means the Receive call is blocked indefinitely.
		_ = ws.Close() // Attempt to close to unblock
		return fmt.Errorf("websocket read timeout after %v (Receive call blocked)", timeout)
	}
}

// Add other shared test helpers here if needed in the future.
