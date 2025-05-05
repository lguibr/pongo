// File: test/e2e_setup_test.go
package test

import (
	"net/http/httptest"
	"strings" // ADDED import
	"testing"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/server"
	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
)

// E2ESetupResult holds the results of the setup function.
type E2ESetupResult struct {
	Engine         *bollywood.Engine
	RoomManagerPID *bollywood.PID
	Server         *httptest.Server
	WsURL          string
	Origin         string
	Cfg            utils.Config
}

// SetupE2ETest initializes the engine, room manager, and test server.
// It accepts a specific config to use.
func SetupE2ETest(t *testing.T, cfg utils.Config) E2ESetupResult {
	t.Helper()

	engine := bollywood.NewEngine()
	roomManagerPID := engine.Spawn(bollywood.NewProps(game.NewRoomManagerProducer(engine, cfg)))
	assert.NotNil(t, roomManagerPID, "RoomManager PID should not be nil")
	time.Sleep(100 * time.Millisecond) // Allow manager to start

	testServer := server.New(engine, roomManagerPID)
	s := httptest.NewServer(websocket.Handler(testServer.HandleSubscribe()))

	wsURL := "ws" + strings.TrimPrefix(s.URL, "http") // Use strings package here
	origin := "http://localhost/" // Standard origin for local tests

	return E2ESetupResult{
		Engine:         engine,
		RoomManagerPID: roomManagerPID,
		Server:         s,
		WsURL:          wsURL,
		Origin:         origin,
		Cfg:            cfg,
	}
}

// TeardownE2ETest shuts down the engine and closes the server.
func TeardownE2ETest(t *testing.T, setupResult E2ESetupResult, shutdownTimeout time.Duration) {
	t.Helper()
	if setupResult.Server != nil {
		setupResult.Server.Close()
	}
	if setupResult.Engine != nil {
		setupResult.Engine.Shutdown(shutdownTimeout)
	}
}