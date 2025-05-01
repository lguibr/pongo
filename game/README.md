
# Game Logic Module

This module contains the core gameplay logic, state management, and actor implementations for the PonGo game, built using the [Bollywood Actor Library](https://github.com/lguibr/bollywood).

## Overview

The game logic is orchestrated by actors. A central RoomManagerActor manages multiple GameActor instances. A temporary ConnectionHandlerActor (in the Server module) manages each WebSocket connection. Each GameActor spawns child actors for game entities and a dedicated BroadcasterActor for state dissemination.

-   **RoomManagerActor**: Manages the lifecycle of GameActor instances. Handles requests (FindRoomRequest) from ConnectionHandlerActor to find or create rooms. Cleans up empty rooms (GameRoomEmpty). Responds to HTTP queries for the room list (`/rooms/`) via GetRoomListRequest using Ask/Reply.
-   **ConnectionHandlerActor (in Server module)**: Manages a single WebSocket connection. Asks RoomManagerActor for a room assignment. Once assigned, communicates *directly* with the designated GameActor to assign the player, forward input, and signal disconnection.
-   **GameActor**: Represents a single game room (up to 4 players). Manages the core game state (canvas, grid, players, scores). Maintains local caches of `Paddle` and `Ball` states. Handles player connections/disconnections (`AssignPlayerToRoom`, `PlayerDisconnect`) *initiated by ConnectionHandlerActor*. Spawns and supervises child actors (`PaddleActor`, `BallActor`) and a `BroadcasterActor`. Drives child actor updates via `UpdatePositionCommand`. Receives `PositionUpdateMessage` from child actors and updates its local state cache. Performs all collision detection and physics calculations using its cached state. Sends commands (`SetVelocity`, `ReflectVelocity`, `SetPhasing`, etc.) to child actors based on collision results. Updates scores and grid state. Handles power-up logic (spawning new balls, sending commands to existing balls). Implements the "persistent ball" logic on player disconnect. Checks for game end condition (all bricks destroyed) and triggers game over sequence (sends `GameOverMessage`, notifies `RoomManagerActor`, stops self). Periodically creates a `GameState` snapshot and sends it (`BroadcastStateCommand`) to its `BroadcasterActor`. Notifies the `RoomManagerActor` when it becomes empty (`GameRoomEmpty`).
-   **BroadcasterActor**: Spawned by GameActor. Maintains the list of active WebSocket connections for its specific room (AddClient, RemoveClient). Receives GameState snapshots (BroadcastStateCommand) from its parent GameActor. Marshals the state to JSON. Sends the JSON payload to all connected clients in its room asynchronously. Handles client send errors and notifies the GameActor of disconnections detected during broadcast. Handles `GameOverMessage` by sending it to all clients and then closing their connections.
-   **Child Actors (PaddleActor, BallActor)**: Manage individual game entities (paddles, balls). Update their state upon receiving `UpdatePositionCommand`. Send `PositionUpdateMessage` back to their parent `GameActor` after updating state. Handle direct commands (e.g., `SetVelocityCommand`, `PaddleDirectionMessage`) from their parent `GameActor`. Respond to `GetPositionRequest` (via `ctx.Reply()`) if needed.
-   **State:** Game state is distributed. RoomManagerActor holds the list of rooms and approximate player counts. Each GameActor holds the authoritative state for its room (players, grid, scores) and caches the last known state of paddles/balls (updated via `PositionUpdateMessage`). BroadcasterActor holds client connections for the room. Child actors manage their local state. ConnectionHandlerActor holds the WebSocket connection reference.
-   **Physics & Rules:** Collision detection (wall, brick, paddle) and response are handled within GameActor using its cached state. Wall collisions now also immediately adjust the ball's position to the boundary to prevent tunneling. Rules for scoring, permanent balls, and power-ups are implemented here, using parameters from `utils/config.go`.
-   **Communication:** Actors communicate via messages defined in `messages.go`. `GameActor` sends state snapshots (`BroadcastStateCommand`) or game over messages (`GameOverMessage`) to `BroadcasterActor`. `BroadcasterActor` sends JSON to clients. `GameActor` commands children (`Send`). Child actors send state updates (`PositionUpdateMessage`) back to `GameActor`. ConnectionHandlerActor interacts with `RoomManagerActor` (initially) and `GameActor` (directly). `RoomManagerActor` replies to `Ask` requests using `ctx.Reply()`.

## Key Components

*   **room_manager.go**: Top-level coordinator (room lifecycle, assignment replies, list replies).
*   **game_actor.go**: Single game room coordinator (physics ticks, state management, child supervision, broadcaster management, state caching, game over logic).
*   **broadcaster_actor.go**: Handles asynchronous broadcasting of game state and game over messages to clients within a room, manages connection cleanup.
*   **paddle_actor.go**: Manages paddle state and movement based on commands, sends state updates.
*   **ball_actor.go**: Manages ball state and movement based on commands, sends state updates.
*   **game_actor_physics.go**: Collision detection logic (uses cached state).
*   **game_actor_handlers.go**: Handlers for GameActor messages (connect/disconnect, state updates).
*   **game_actor_broadcast.go**: Contains `createGameStateSnapshot` helper function.
*   **paddle.go, ball.go, etc.**: Data structures.
*   **messages.go**: Defines all actor message types.

## Related Modules

*   [Bollywood Actor Library](https://github.com/lguibr/bollywood) (External Dependency)
*   [Server](../server/README.md) (Contains ConnectionHandlerActor)
*   [Utilities](../utils/README.md) (Contains config.go)
*   [Main Project](../README.md)