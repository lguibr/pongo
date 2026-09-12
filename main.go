package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/server"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// checkOrigin rejects a malformed Origin header; any well-formed origin passes. A
// request without Origin passes here, but the x/net handshake then refuses it with 403.
func checkOrigin(config *websocket.Config, req *http.Request) (err error) {
	config.Origin, err = websocket.Origin(config, req)
	return err
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel()})))

	cfg := utils.DefaultConfig()
	slog.Info("starting", "canvasSize", cfg.CanvasSize, "gridSize", cfg.GridSize, "tick", cfg.GameTickPeriod, "broadcastHz", cfg.BroadcastRateHz)

	engine := actor.NewEngine()
	roomManagerPID := engine.Spawn(actor.NewProps(game.NewRoomManagerProducer(engine, cfg)))
	if roomManagerPID == nil {
		panic("failed to spawn room manager")
	}
	websocketServer := server.New(engine, roomManagerPID)

	http.HandleFunc("/", server.HandleHealthCheck())
	http.HandleFunc("/health-check/", server.HandleHealthCheck())
	http.HandleFunc("/rooms/", websocketServer.HandleGetRooms())

	subscribeHandler := websocket.Handler(websocketServer.HandleSubscribe())
	http.HandleFunc("/subscribe", func(w http.ResponseWriter, req *http.Request) {
		// Cloud Run terminates TLS and reports the original scheme in X-Forwarded-Proto.
		scheme, originScheme := "ws", "http"
		if req.TLS != nil || strings.EqualFold(req.Header.Get("X-Forwarded-Proto"), "https") {
			scheme, originScheme = "wss", "https"
		}
		location := fmt.Sprintf("%s://%s/subscribe", scheme, req.Host)
		origin := req.Header.Get("Origin")
		if origin == "" {
			origin = fmt.Sprintf("%s://%s", originScheme, req.Host)
		}
		config, err := websocket.NewConfig(location, origin)
		if err != nil {
			slog.Warn("invalid websocket config", "location", location, "origin", origin, "err", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if err := checkOrigin(config, req); err != nil {
			slog.Warn("websocket origin rejected", "origin", req.Header.Get("Origin"), "err", err)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		subscribeHandler.ServeHTTP(w, req)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Cloud Run sets PORT; 8080 is its default
	}
	listenAddr := ":" + port
	// net/http reports accept and handler failures through ErrorLog; keep them at error level.
	httpServer := &http.Server{Addr: listenAddr, ReadHeaderTimeout: 5 * time.Second, ErrorLog: slog.NewLogLogger(slog.Default().Handler(), slog.LevelError)}
	shutdown, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.ListenAndServe() }()
	slog.Info("listening", "addr", listenAddr)
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server stopped", "err", err)
		}
	case <-shutdown.Done():
		slog.Info("shutting down")
	}
	// HTTP shutdown does not own hijacked WebSockets; stop room/connection actors first.
	engine.Shutdown(5 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("http shutdown", "err", err)
	}
}

// logLevel reads PONGO_LOG_LEVEL (debug, info, warn or error); the default is info.
func logLevel() slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(os.Getenv("PONGO_LOG_LEVEL"))); err != nil {
		return slog.LevelInfo
	}
	return level
}
