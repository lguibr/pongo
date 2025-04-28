// File: main.go
package main

import (
	"fmt"
	"net/http" // Import url package
	// Import strings package
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/server"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

const servicePort = "8080"

// --- ADDED: Function to check origin (ALLOWS ALL - USE CAUTIOUSLY) ---
func checkOrigin(config *websocket.Config, req *http.Request) (err error) {
	// This function bypasses the default origin check.
	// Consider adding more specific checks if needed for security.
	// For example, allow only specific origins:
	/*
	   origin := req.Header.Get("Origin")
	   allowedOrigins := []string{"http://localhost:5173", "https://your-frontend.com"}
	   isAllowed := false
	   for _, allowed := range allowedOrigins {
	       if origin == allowed {
	           isAllowed = true
	           break
	       }
	   }
	   if !isAllowed {
	       return fmt.Errorf("origin %q not allowed", origin)
	   }
	*/
	fmt.Printf("Bypassing origin check for Origin: %s, Host: %s\n", req.Header.Get("Origin"), req.Host) // Log bypass
	config.Origin, err = websocket.Origin(config, req)
	if err == nil && config.Origin == nil {
		// If Origin header is not present, default check might allow based on Host.
		// We can explicitly create a default Origin based on the Host if needed,
		// but often just returning nil error here is sufficient if Origin header is missing.
		// Let's try returning nil first. If issues persist, construct origin from Host.
		fmt.Println("Origin header not present, allowing connection.")
		// Example if needed:
		// config.Origin = &url.URL{Scheme: "http", Host: req.Host} // Or https based on req scheme
	} else if err != nil {
		fmt.Printf("Error checking origin (websocket.Origin): %v\n", err)
		// Return the error from websocket.Origin if it occurred
		return err
	}
	// If websocket.Origin succeeded or we explicitly allowed nil Origin, return nil error
	return nil
}

// --- END ADD ---

func main() {
	fmt.Println(">>> RUNNING CODE VERSION: [Lucas-Apr28-v1.1.1-OriginFixAttempt] <<<")

	// ... (rest of your setup code: config, engine, roomManager, server) ...
	cfg := utils.DefaultConfig()
	fmt.Println("Configuration loaded (using defaults).")
	fmt.Printf("Canvas Size: %d, Grid Size: %d, Tick Period: %v\n", cfg.CanvasSize, cfg.GridSize, cfg.GameTickPeriod)

	engine := bollywood.NewEngine()
	fmt.Println("Bollywood Engine created.")

	roomManagerProps := bollywood.NewProps(game.NewRoomManagerProducer(engine, cfg))
	roomManagerPID := engine.Spawn(roomManagerProps)
	if roomManagerPID == nil {
		panic("Failed to spawn RoomManagerActor")
	}
	fmt.Printf("RoomManagerActor spawned with PID: %s\n", roomManagerPID)

	time.Sleep(50 * time.Millisecond)

	websocketServer := server.New(engine, roomManagerPID)
	fmt.Println(">>> DEBUG: server.New() completed.")
	fmt.Println("WebSocket Server created.")

	// 4. Setup Handlers
	http.HandleFunc("/", server.HandleHealthCheck())
	http.HandleFunc("/health-check/", server.HandleHealthCheck())
	http.HandleFunc("/rooms/", websocketServer.HandleGetRooms())

	// --- MODIFIED: Use websocket.Server with custom Config ---
	wsServer := websocket.Server{
		Handler: websocket.Handler(websocketServer.HandleSubscribe()),
		// Handshake: checkOrigin, // Use the custom origin check function
		// Let's try configuring the Config directly within the handler setup
	}
	// http.Handle("/subscribe", wsServer) // Use the configured server

	// --- Alternative/Simpler Modification: Configure Handler directly ---
	subscribeHandler := websocket.Handler(websocketServer.HandleSubscribe())
	http.HandleFunc("/subscribe", func(w http.ResponseWriter, req *http.Request) {
		// Create a custom Config for this connection attempt
		config, err := websocket.NewConfig(fmt.Sprintf("wss://%s/subscribe", req.Host), fmt.Sprintf("https://%s", req.Host)) // Use wss/https based on expected deployment
		if err != nil {
			fmt.Printf("Error creating websocket config: %v\n", err)
			http.Error(w, "Internal Server Error", 500)
			return
		}
		// Perform the custom origin check
		err = checkOrigin(config, req) // Call our custom check
		if err != nil {
			fmt.Printf("Origin check failed: %v\n", err)
			http.Error(w, "Forbidden", 403)
			return
		}
		// If origin check passes, serve the actual handler
		fmt.Println("Origin check passed, serving WebSocket handler.")
		subscribeHandler.ServeHTTP(w, req)
	})
	// --- END MODIFICATION ---

	fmt.Println(">>> DEBUG: HTTP Handlers registered.")

	// 5. Determine Port and Start Server
	listenAddr := ":" + servicePort
	fmt.Printf(">>> DEBUG: Attempting to listen on address: [%s] <<<\n", listenAddr)
	fmt.Printf("Server starting explicitly on address %s\n", listenAddr)
	err := http.ListenAndServe(listenAddr, nil)
	if err != nil {
		fmt.Printf(">>> DEBUG: http.ListenAndServe on %s failed: %v <<<\n", listenAddr, err)
		fmt.Println("Server stopped:", err)
		fmt.Println("Shutting down engine...")
		engine.Shutdown(5 * time.Second)
		fmt.Println("Engine shutdown complete.")
	}
}
