package main

import (
	"fmt"
	"net/http"
	"net/url" // Import url package
	"strings" // Import strings package
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/server"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

const servicePort = "8080" // Hardcoded port for Cloud Run

// --- Function to check origin ---
func checkOrigin(config *websocket.Config, req *http.Request) (err error) {
	// host := req.Host // Removed unused variable
	// fmt.Printf("Origin Check: Header Origin='%s', Request Host='%s'\n", origin, host) // Removed log

	// If Origin header is present, let websocket.Origin perform the default check.
	// If it's missing, we might allow based on Host or other criteria.
	config.Origin, err = websocket.Origin(config, req)
	if err == nil {
		if config.Origin == nil {
			// websocket.Origin returns nil origin and nil error if Origin header is missing.
			// This is often allowed by default browsers for ws:// connections from http:// origins.
			// For Cloud Run (HTTPS -> HTTP), the Origin header *should* be present.
			// If it's missing, it might indicate a non-browser client or misconfiguration.
			// We'll allow it for now but log a warning.
			fmt.Println("Origin Check: Origin header missing, allowing connection (check client/proxy config).")
			// Optionally, construct a default origin based on Host if strict checking is needed:
			// defaultOriginUrl := &url.URL{Scheme: "http", Host: req.Host} // Adjust scheme as needed
			// config.Origin = defaultOriginUrl
			return nil // Allow connection
		}
		// Origin header was present and matched the config's expected origin.
		// fmt.Printf("Origin Check: websocket.Origin check passed for origin %s\n", config.Origin) // Removed log
		return nil // Origin check passed
	}

	// websocket.Origin returned an error (likely origin mismatch)
	fmt.Printf("Origin Check: websocket.Origin check failed: %v\n", err)

	// --- Custom Allow Logic (Example - USE WITH CAUTION) ---
	// Allow specific origins explicitly if needed, bypassing the standard check error.
	// This is generally NOT recommended for security unless you have specific needs.
	/*
	   allowedOrigins := []string{"http://localhost:5173", "https://your-frontend-domain.com"}
	   requestOrigin := req.Header.Get("Origin") // Get the actual Origin header value
	   isAllowed := false
	   for _, allowed := range allowedOrigins {
	       if requestOrigin == allowed {
	           isAllowed = true
	           break
	       }
	   }
	   if isAllowed {
	       fmt.Printf("Origin Check: Custom allow for origin %s\n", requestOrigin)
	       config.Origin, _ = url.Parse(requestOrigin) // Set the config origin to the allowed one
	       return nil // Bypass the original error
	   }
	*/
	// --- End Custom Allow Logic ---

	// If no custom logic allowed it, return the original error from websocket.Origin
	return err
}

func main() {
	// --- ADD A UNIQUE MARKER TO CONFIRM THIS VERSION IS RUNNING ---
	fmt.Println(">>> RUNNING CODE VERSION: [LG-Apr28-v1.1.4-CleanupRefactor] <<<")
	// --- END MARKER ---

	// 0. Load Configuration
	cfg := utils.DefaultConfig()
	fmt.Println("Configuration loaded (using defaults).")
	fmt.Printf("Canvas Size: %d, Grid Size: %d, Tick Period: %v, Broadcast Rate: %dHz\n",
		cfg.CanvasSize, cfg.GridSize, cfg.GameTickPeriod, cfg.BroadcastRateHz)

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
	fmt.Println("WebSocket Server instance created.")

	// 4. Setup Handlers
	http.HandleFunc("/", server.HandleHealthCheck())              // Simple health check at root
	http.HandleFunc("/health-check/", server.HandleHealthCheck()) // Explicit health check endpoint
	http.HandleFunc("/rooms/", websocketServer.HandleGetRooms())  // Get room list

	// --- MODIFIED: Use http.HandleFunc with custom origin check ---
	subscribeHandler := websocket.Handler(websocketServer.HandleSubscribe())
	http.HandleFunc("/subscribe", func(w http.ResponseWriter, req *http.Request) {
		// Determine scheme based on request or Cloud Run headers
		scheme := "ws"
		originScheme := "http"
		if req.TLS != nil || strings.EqualFold(req.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "wss"
			originScheme = "https"
		}
		// fmt.Printf("Handle /subscribe: Detected scheme: %s (Origin scheme: %s)\n", scheme, originScheme) // Removed log

		// Construct URLs carefully using the Host header
		// Location URL (where the WebSocket endpoint is)
		locationUrlStr := fmt.Sprintf("%s://%s/subscribe", scheme, req.Host)
		// Origin URL (where the request is coming from - usually without path)
		// Use the Origin header if present, otherwise construct from Host.
		originUrlStr := req.Header.Get("Origin")
		if originUrlStr == "" {
			// Construct a default origin if header is missing
			originUrlStr = fmt.Sprintf("%s://%s", originScheme, req.Host)
			// fmt.Printf("Handle /subscribe: Origin header missing, using constructed origin: %s\n", originUrlStr) // Removed log
		}

		// Validate constructed URLs before creating config
		_, errLoc := url.Parse(locationUrlStr)
		_, errOrg := url.Parse(originUrlStr)
		if errLoc != nil || errOrg != nil {
			fmt.Printf("Error parsing URLs for websocket config (Location: '%s', Origin: '%s'): LocErr=%v, OrgErr=%v\n", locationUrlStr, originUrlStr, errLoc, errOrg)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Create WebSocket config
		config, err := websocket.NewConfig(locationUrlStr, originUrlStr)
		if err != nil {
			fmt.Printf("Error creating websocket config (Location: %s, Origin: %s): %v\n", locationUrlStr, originUrlStr, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Perform the custom origin check
		err = checkOrigin(config, req) // Call our custom check
		if err != nil {
			fmt.Printf("Origin check failed for Origin '%s' against config Origin '%s': %v\n", req.Header.Get("Origin"), config.Origin, err)
			http.Error(w, "Forbidden", http.StatusForbidden) // Use constant
			return
		}

		// If origin check passes, serve the actual handler using the original Handler interface
		// fmt.Printf("Origin check passed for Origin '%s', serving WebSocket handler.\n", config.Origin) // Removed log
		subscribeHandler.ServeHTTP(w, req) // Use the original handler
	})
	// --- END MODIFICATION ---

	fmt.Println("HTTP Handlers registered.")

	// 5. Determine Port and Start Server
	listenAddr := ":" + servicePort
	fmt.Printf("Server starting on address %s\n", listenAddr)
	err := http.ListenAndServe(listenAddr, nil)
	if err != nil {
		fmt.Printf("FATAL: http.ListenAndServe on %s failed: %v\n", listenAddr, err)
		// Handle shutdown gracefully
		fmt.Println("Shutting down engine...")
		engine.Shutdown(5 * time.Second) // Allow actors time to stop
		fmt.Println("Engine shutdown complete.")
	}
}
