
# Server Module

This module handles the initial setup of the HTTP server and WebSocket endpoint for the PonGo game. It spawns a dedicated actor (ConnectionHandlerActor) for each connection, which then interacts with the game's actor system.

## Overview

-   **server/websocket.go**: Defines the Server struct which holds references to the actor Engine and the RoomManagerActor PID.
-   **server/handlers.go**:
    -   `HandleSubscribe`: Accepts new WebSocket connections (`/subscribe`). Spawns a ConnectionHandlerActor to manage the connection lifecycle.
    -   `HandleGetRooms`: Handles HTTP GET requests to `/rooms/`. Queries the RoomManagerActor using Engine.Ask to get a list of active rooms and their player counts, returning it as JSON.
    -   `HandleHealthCheck`: Handles HTTP GET requests to `/` and `/health-check/`. Returns a simple `{"status": "ok"}` JSON response.
-   **server/connection_handler.go**:
    -   Defines ConnectionHandlerActor, spawned per connection.
    -   On start, asks the RoomManagerActor for a room assignment (FindRoomRequest -> AssignRoomResponse).
    -   Once assigned a GameActor PID, sends AssignPlayerToRoom *directly* to that GameActor.
    -   Contains the readLoop for the WebSocket connection.
    -   Forwards incoming player input (ForwardedPaddleDirection) *directly* to the assigned GameActor.
    -   Sends PlayerDisconnect *directly* to the assigned GameActor when the connection closes or errors.
    -   Stops itself when the connection is terminated.

## Key Interactions

1.  **New Connection:** Client connects -> HandleSubscribe -> Spawns ConnectionHandlerActor.
2.  **Room Assignment:** ConnectionHandlerActor -> FindRoomRequest to RoomManagerActor -> AssignRoomResponse back to ConnectionHandlerActor.
3.  **Player Assignment:** ConnectionHandlerActor -> AssignPlayerToRoom to assigned GameActor.
4.  **Player Input:** Client sends input -> ConnectionHandlerActor's readLoop -> ForwardedPaddleDirection *directly* to assigned GameActor.
5.  **Disconnect:** Connection closes/errors -> ConnectionHandlerActor's readLoop exits -> PlayerDisconnect *directly* to assigned GameActor -> ConnectionHandlerActor stops.
6.  **HTTP Room List Query:** Client GET `/rooms/` -> HandleGetRooms -> Engine.Ask(RoomManagerPID, GetRoomListRequest) -> RoomManagerActor replies -> JSON response to client.
7.  **HTTP Health Check:** Client GET `/` or `/health-check/` -> HandleHealthCheck -> JSON response `{"status": "ok"}`.

## Related Modules

*   [Bollywood Actor Library](https://github.com/lguibr/bollywood) (External Dependency)
*   [Game Logic](../game/README.md) (Contains RoomManagerActor, GameActor, BroadcasterActor)
*   [Utilities](../utils/README.md)
*   [Main Project](../README.md)