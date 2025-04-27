# File: server/README.md
# Server Module

This module handles the initial setup of the HTTP server and WebSocket endpoint for the PonGo game. Its primary responsibility is to accept incoming connections and forward them to the central [GameActor](../game/README.md) for management.

## Responsibilities

*   Listens for HTTP connections on the specified port (default 3001).
*   Handles the `/subscribe` WebSocket endpoint.
*   Accepts new WebSocket connections.
*   Forwards new connections (`PlayerConnection` interface) to the `GameActor` via a `PlayerConnectRequest` message.
*   Starts a `readLoop` goroutine for each connection to listen for incoming messages (e.g., player input).
*   Forwards valid player input messages to the `GameActor` via a `ForwardedPaddleDirection` message.
*   Detects WebSocket read errors or closures (EOF) in the `readLoop`.
*   Sends a `PlayerDisconnect` message (containing the specific `PlayerConnection`) to the `GameActor` when a read error or closure occurs.
*   Provides an HTTP GET endpoint (`/`) for basic status or potentially retrieving cached game state (implementation may vary).

## Key Components

*   **`Server` (`websocket.go`)**: Holds references to the `Bollywood` engine and the `GameActor`'s PID. Does *not* actively manage the list of connections itself.
*   **`HandleSubscribe` (`handlers.go`)**: The WebSocket connection handler function.
*   **`readLoop` (`handlers.go`)**: Goroutine responsible for reading from a single WebSocket connection and signaling disconnects.
*   **`HandleGetSit` (`handlers.go`)**: The HTTP GET handler.

## Connection Lifecycle (Simplified)

1.  Client connects to `/subscribe`.
2.  `HandleSubscribe` accepts the `websocket.Conn`.
3.  `HandleSubscribe` sends `PlayerConnectRequest{WsConn: ws}` to `GameActor`.
4.  `HandleSubscribe` starts `readLoop(ws)`.
5.  `readLoop` reads messages:
    *   Valid input -> Sends `ForwardedPaddleDirection{WsConn: ws, ...}` to `GameActor`.
    *   Read Error/EOF -> Sends `PlayerDisconnect{WsConn: ws}` to `GameActor` -> `readLoop` exits.
6.  `GameActor` handles connect/disconnect logic, including closing the WebSocket connection when a player is fully removed.

## Related Modules

*   [Bollywood Actor Library](../bollywood/README.md)
*   [Game Logic](../game/README.md)
*   [Main Project](../README.md)