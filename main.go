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

const servicePort = "8080" // Hardcoded port for Cloud Run

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
	// --- ADD A UNIQUE MARKER TO CONFIRM THIS VERSION IS RUNNING ---
	fmt.Println(">>> RUNNING CODE VERSION: [Lucas-Apr28-v1.1.3-OriginFixAttempt] <<<")
	// --- END MARKER ---

	// 0. Load Configuration
	cfg := utils.DefaultConfig()
	fmt.Println("Configuration loaded (using defaults).")
	fmt.Printf("Canvas Size: %d, Grid Size: %d, Tick Period: %v\n", cfg.CanvasSize, cfg.GridSize, cfg.GameTickPeriod)

	// 1. Initialize Bollywood Engine
	engine := bollywood.NewEngine()
	fmt.Println("Bollywood Engine created.")

	// 2. Spawn the RoomManagerActor, passing the config
	roomManagerProps := bollywood.NewProps(game.NewRoomManagerProducer(engine, cfg)) // Pass cfg
	roomManagerPID := engine.Spawn(roomManagerProps)
	if roomManagerPID == nil {
		panic("Failed to spawn RoomManagerActor")
	}
	fmt.Printf("RoomManagerActor spawned with PID: %s\n", roomManagerPID)

	// Allow RoomManagerActor to start
	time.Sleep(50 * time.Millisecond)

	// 3. Create the HTTP/WebSocket Server
	websocketServer := server.New(engine, roomManagerPID) // Pass RoomManager PID
	fmt.Println(">>> DEBUG: server.New() completed.")     // Debug log
	fmt.Println("WebSocket Server created.")              // Original log

	// 4. Setup Handlers
	http.HandleFunc("/", server.HandleHealthCheck())              // Simple health check at root
	http.HandleFunc("/health-check/", server.HandleHealthCheck()) // Explicit health check endpoint
	http.HandleFunc("/rooms/", websocketServer.HandleGetRooms())  // Get room list

	// --- MODIFIED: Use http.HandleFunc with custom origin check ---
	subscribeHandler := websocket.Handler(websocketServer.HandleSubscribe())
	http.HandleFunc("/subscribe", func(w http.ResponseWriter, req *http.Request) {
		// Determine scheme based on request or Cloud Run headers if needed
		// Cloud Run terminates TLS, so the request Go sees might be HTTP,
		// but we need to construct the config URLs based on the external access (HTTPS).
		// X-Forwarded-Proto is usually reliable.
		scheme := "wss" // Assume HTTPS for Cloud Run external access
		originScheme := "https"
		if req.Header.Get("X-Forwarded-Proto") == "http" {
			// This case should be rare for Cloud Run default URLs but handles potential non-HTTPS setups
			scheme = "ws"
			originScheme = "http"
		}

		// Construct URLs carefully using the Host header
		requestUrl := fmt.Sprintf("%s://%s/subscribe", scheme, req.Host)
		// Origin URL should generally not include the path
		originUrl := fmt.Sprintf("%s://%s", originScheme, req.Host)

		config, err := websocket.NewConfig(requestUrl, originUrl)
		if err != nil {
			fmt.Printf("Error creating websocket config (URL: %s, Origin: %s): %v\n", requestUrl, originUrl, err)
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

		// If origin check passes, serve the actual handler using the original Handler interface
		fmt.Println("Origin check passed, serving WebSocket handler.")
		subscribeHandler.ServeHTTP(w, req) // Use the original handler
	})
	// --- END MODIFICATION ---

	fmt.Println(">>> DEBUG: HTTP Handlers registered.") // Debug log

	// 5. Determine Port and Start Server
	listenAddr := ":" + servicePort                                                  // Use the hardcoded port
	fmt.Printf(">>> DEBUG: Attempting to listen on address: [%s] <<<\n", listenAddr) // Debug log
	fmt.Printf("Server starting explicitly on address %s\n", listenAddr)             // Original log
	err := http.ListenAndServe(listenAddr, nil)
	if err != nil {
		fmt.Printf(">>> DEBUG: http.ListenAndServe on %s failed: %v <<<\n", listenAddr, err) // Debug log
		// Handle shutdown gracefully
		fmt.Println("Server stopped:", err)
		fmt.Println("Shutting down engine...")
		engine.Shutdown(5 * time.Second) // Allow actors time to stop
		fmt.Println("Engine shutdown complete.")
	}
}
