# Server Module

This module handles the initial setup of the HTTP server and WebSocket endpoint for the PonGo game. It spawns a dedicated actor (ConnectionHandlerActor) for each connection, which then interacts with the game's actor system.

## Overview

-   **server/websocket.go**: Defines the Server struct which holds references to the actor Engine and the RoomManagerActor PID.
-   **server/handlers.go**:
    -   `HandleSubscribe`: Accepts new WebSocket connections (`/subscribe`). Spawns a ConnectionHandlerActor to manage the connection lifecycle. Waits for the ConnectionHandlerActor to signal completion before returning, ensuring resources are managed.
    -   `HandleGetRooms`: Handles HTTP GET requests to `/rooms/`. Queries the RoomManagerActor using Engine.Ask to get a list of active rooms and their player counts, returning it as JSON.
    -   `HandleHealthCheck`: Handles HTTP GET requests to `/` and `/health-check/`. Returns a simple `{"status": "ok"}` JSON response.
-   **server/connection_handler.go**:
    -   Defines ConnectionHandlerActor, spawned per connection.
    -   On start, asks the RoomManagerActor for a room assignment (FindRoomRequest -> AssignRoomResponse).
    -   Once assigned a GameActor PID, sends AssignPlayerToRoom *directly* to that GameActor.
    -   Contains the `readLoop` for the WebSocket connection. Signals its exit to the main actor loop via an error message.
    -   Forwards incoming player input (ForwardedPaddleDirection) *directly* to the assigned GameActor.
    -   Sends PlayerDisconnect *directly* to the assigned GameActor when the connection closes or errors (detected by `readLoop` or `Stopping` handler).
    -   Stops itself when the connection is terminated or the actor is stopped by the server. Ensures `readLoop` is stopped before fully exiting.
    -   Signals completion to the `HandleSubscribe` handler via a `done` channel when the actor fully stops.

## Key Interactions

1.  **New Connection:** Client connects -> HandleSubscribe -> Spawns ConnectionHandlerActor.
2.  **Room Assignment:** ConnectionHandlerActor -> FindRoomRequest to RoomManagerActor -> AssignRoomResponse back to ConnectionHandlerActor.
3.  **Player Assignment:** ConnectionHandlerActor -> AssignPlayerToRoom to assigned GameActor. GameActor sends initial messages (`PlayerAssignmentMessage`, `InitialGridStateMessage`) directly to client.
4.  **Player Input:** Client sends input -> ConnectionHandlerActor's readLoop -> ForwardedPaddleDirection *directly* to assigned GameActor.
5.  **Game State Updates:** GameActor generates atomic updates -> Sends `BroadcastUpdatesCommand` to BroadcasterActor -> BroadcasterActor sends `GameUpdatesBatch` JSON to all clients in the room.
6.  **Disconnect:** Connection closes/errors -> ConnectionHandlerActor's readLoop exits -> Sends error to self -> Cleanup logic runs -> PlayerDisconnect *directly* to assigned GameActor -> ConnectionHandlerActor stops -> Closes `done` channel.
7.  **HTTP Room List Query:** Client GET `/rooms/` -> HandleGetRooms -> Engine.Ask(RoomManagerPID, GetRoomListRequest) -> RoomManagerActor replies -> JSON response to client.
8.  **HTTP Health Check:** Client GET `/` or `/health-check/` -> HandleHealthCheck -> JSON response `{"status": "ok"}`.

## Related Modules

*   [Bollywood Actor Library](https://github.com/lguibr/bollywood) (External Dependency)
*   [Game Logic](../game/README.md) (Contains RoomManagerActor, GameActor, BroadcasterActor)
*   [Utilities](../utils/README.md)
*   [Main Project](../README.md)
