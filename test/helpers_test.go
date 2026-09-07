// File: test/helpers.go
package test

import (
	"errors"
	"fmt"
	"golang.org/x/net/websocket"
	"testing"
	"time"
)

// ReadWsJSONMessage reads a JSON message from the WebSocket with a timeout.
// It handles setting/clearing read deadlines and checks for common errors.
// Renamed to be exported.
func ReadWsJSONMessage(t *testing.T, ws *websocket.Conn, timeout time.Duration, v interface{}) error {
	t.Helper()
	if ws == nil {
		return errors.New("nil websocket")
	}
	if err := ws.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	defer ws.SetReadDeadline(time.Time{})
	return websocket.JSON.Receive(ws, v)
}

// quickPlayHandshake consumes only the room response; assignment and initial
// state remain for each test to assert through the actual protocol.
func quickPlayHandshake(t *testing.T, ws *websocket.Conn) {
	t.Helper()
	request := map[string]string{"messageType": "quickPlay", "sessionId": fmt.Sprintf("test-%d", time.Now().UnixNano())}
	if err := websocket.JSON.Send(ws, request); err != nil {
		t.Fatal(err)
	}
	var reply struct {
		MessageType string `json:"messageType"`
		Success     bool   `json:"success"`
		Reason      string `json:"reason"`
	}
	if err := ReadWsJSONMessage(t, ws, 5*time.Second, &reply); err != nil {
		t.Fatal(err)
	}
	if reply.MessageType != "roomJoined" || !reply.Success {
		t.Fatalf("admission failed: %+v", reply)
	}
}
