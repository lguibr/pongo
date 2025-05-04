# File: game/README.md

# Game Logic Module

This module contains the core gameplay logic, state management, and actor implementations for the PonGo game, built using the [Bollywood Actor Library](https://github.com/lguibr/bollywood). It features a decoupled architecture where high-frequency physics simulation is separated from fixed-rate network broadcasting.

## Overview

The game logic is orchestrated by actors. A central `RoomManagerActor` manages multiple `GameActor` instances. A temporary `ConnectionHandlerActor` (in the Server module) manages each WebSocket connection. Each `GameActor` spawns child actors for game entities (`PaddleActor`, `BallActor`) and a dedicated `BroadcasterActor` for state dissemination.

-   **RoomManagerActor**: Manages the lifecycle of `GameActor` instances. Handles requests (`FindRoomRequest`) from `ConnectionHandlerActor` to find or create rooms. Cleans up empty or finished rooms (`GameRoomEmpty`). Responds to HTTP queries for the room list (`/rooms/`) via `GetRoomListRequest` using Ask/Reply.
-   **ConnectionHandlerActor (in Server module)**: Manages a single WebSocket connection. Asks `RoomManagerActor` for a room assignment. Once assigned, communicates *directly* with the designated `GameActor` to assign the player, forward input, and signal disconnection.
-   **GameActor**: Represents a single game room (up to 4 players). Manages the core game state (canvas, grid, players, scores). Maintains the **authoritative local cache** of `Paddle` and `Ball` states (position, velocity, etc.), including temporary `Collided` flags. Handles player connections/disconnections (`AssignPlayerToRoom`, `PlayerDisconnect`) *initiated by ConnectionHandlerActor*. Sends initial state (`PlayerAssignmentMessage`, `InitialGridStateMessage`) directly to the connecting client. Spawns and supervises child actors (`PaddleActor`, `BallActor`) and a `BroadcasterActor`.
    -   **Physics Simulation (High Frequency):** Runs an internal `physicsTicker` (e.g., 60Hz). On each `GameTick`, it updates the positions of paddles and balls in its local cache (`updateInternalState`), performs collision detection using this cache (`detectCollisions`), updates scores/grid, handles power-ups, and sends **commands** (`SetVelocity`, `ReflectVelocity`, `SetPhasing`, `PaddleDirectionMessage`, etc.) to child actors to modify their internal state for the *next* tick.
    -   **State Broadcasting (Fixed Rate):** Runs an internal `broadcastTicker` (e.g., 30Hz). On each `BroadcastTick`, it creates a `GameState` snapshot from its local cache (including `Collided` flags), resets the `Collided` flags in the cache, and sends the snapshot (`BroadcastStateCommand`) to its `BroadcasterActor`.
    -   **Game Logic:** Implements scoring rules, "hit own wall" logic (score penalty, lose ownership), "persistent ball" logic on player disconnect, and checks for game end condition (all bricks destroyed), triggering the game over sequence (`GameOverMessage`, notifies `RoomManagerActor`, stops self).
-   **BroadcasterActor**: Spawned by `GameActor`. Maintains the list of active WebSocket connections for its specific room (`AddClient`, `RemoveClient`). Receives `GameState` snapshots (`BroadcastStateCommand`) from its parent `GameActor`. Marshals the state (including `Collided` flags) to JSON. Sends the JSON payload to all connected clients in its room asynchronously. Handles client send errors and notifies the `GameActor` of disconnections detected during broadcast. Handles `GameOverMessage` by sending it to all clients and then closing their connections.
-   **Child Actors (PaddleActor, BallActor)**: Manage internal state based on commands received from their parent `GameActor`.
    -   `PaddleActor`: Updates its internal `Direction` based on `PaddleDirectionMessage`.
    -   `BallActor`: Updates its internal velocity, phasing status, mass, etc., based on commands like `SetVelocityCommand`, `ReflectVelocityCommand`, `SetPhasingCommand`.
    -   **Crucially, child actors DO NOT update their own positions based on a timer and DO NOT send position updates back to the `GameActor`.**
-   **State:** Game state is distributed. `RoomManagerActor` holds the list of rooms. Each `GameActor` holds the authoritative state for its room (players, grid, scores) and the **authoritative cache** of paddle/ball states used for simulation and broadcasting. `BroadcasterActor` holds client connections. Child actors manage their specific internal state (like velocity or direction) but not their absolute position over time. The `GameState` snapshot sent to clients includes the dynamic state (players, paddles, balls with `Collided` flags) needed for rendering. The static grid is sent once initially.
-   **Physics & Rules:** Collision detection (wall, brick, paddle) and response are handled within `GameActor` using its authoritative cache during the `GameTick`. Wall collisions immediately adjust the ball's position in the cache. Commands are sent to child actors to update their velocity/phasing for subsequent ticks. Rules for scoring, permanent balls, power-ups, and game completion are implemented here, using parameters from `utils/config.go`. Collision flags (`Collided`) are set in the `GameActor`'s cache during collision detection and included in the `GameState` broadcast for one broadcast tick.
-   **Communication:** `GameActor` sends state snapshots (`BroadcastStateCommand`) or game over messages (`GameOverMessage`) to `BroadcasterActor`. `BroadcasterActor` sends JSON to clients. `GameActor` sends **commands** to children (`Send`). `ConnectionHandlerActor` interacts with `RoomManagerActor` (initially) and `GameActor` (directly). `RoomManagerActor` replies to `Ask` requests using `ctx.Reply()`. **`PositionUpdateMessage` is no longer used.**

## Key Components

*   **room_manager.go**: Top-level coordinator (room lifecycle, assignment replies, list replies).
*   **game_actor.go**: Single game room coordinator (physics ticks, broadcast ticks, authoritative state cache, child supervision, broadcaster management, game over logic).
*   **broadcaster_actor.go**: Handles asynchronous broadcasting of game state and game over messages to clients within a room, manages connection cleanup.
*   **paddle_actor.go**: Manages internal paddle state (`Direction`) based on commands.
*   **ball_actor.go**: Manages internal ball state (velocity, phasing, mass) based on commands.
*   **game_actor_physics.go**: Collision detection logic (uses `GameActor` cache, sets internal `Collided` flags, handles own-wall hits, sends commands to children).
*   **game_actor_handlers.go**: Handlers for `GameActor` messages (connect/disconnect, input forwarding, ball spawning/destruction).
*   **game_actor_broadcast.go**: Contains `createGameStateSnapshot` helper function (reads cache, copies `Collided` flags, resets internal flags).
*   **paddle.go, ball.go, etc.**: Data structures (include `Collided` field). `Move()` methods are now primarily called by `GameActor` on cached objects.
*   **messages.go**: Defines all actor message types (excluding the removed `PositionUpdateMessage`).
*   **grid.go, cell.go**: Grid and Cell data structures and grid generation logic (`Fill`).

## Related Modules

*   [Bollywood Actor Library](https://github.com/lguibr/bollywood) (External Dependency)
*   [Server](../server/README.md) (Contains ConnectionHandlerActor)
*   [Utilities](../utils/README.md) (Contains config.go)
*   [Main Project](../README.md)