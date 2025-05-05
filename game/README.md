# File: game/README.md
# Game Logic Module

This module contains the core gameplay logic, state management, and actor implementations for the PonGo game, built using the [Bollywood Actor Library](https://github.com/lguibr/bollywood). It features a decoupled architecture where high-frequency physics simulation is separated from fixed-rate network broadcasting of atomic state changes.

## Overview

The game logic is orchestrated by actors. A central `RoomManagerActor` manages multiple `GameActor` instances. A temporary `ConnectionHandlerActor` (in the Server module) manages each WebSocket connection. Each `GameActor` spawns child actors for game entities (`PaddleActor`, `BallActor`) and a dedicated `BroadcasterActor` for state dissemination.

-   **RoomManagerActor**: Manages the lifecycle of `GameActor` instances. Handles requests (`FindRoomRequest`) from `ConnectionHandlerActor` to find or create rooms. Cleans up empty or finished rooms (`GameRoomEmpty`) by stopping the corresponding `GameActor`. Responds to HTTP queries for the room list (`/rooms/`) via `GetRoomListRequest` using Ask/Reply.
-   **ConnectionHandlerActor (in Server module)**: Manages a single WebSocket connection. Asks `RoomManagerActor` for a room assignment. Once assigned, communicates *directly* with the designated `GameActor` to assign the player, forward input, and signal disconnection. Ensures its internal `readLoop` is stopped before the actor fully terminates.
-   **GameActor**: Represents a single game room (up to 4 players). Manages the core game state (canvas, grid, players, scores). Maintains the **authoritative local cache** of `Paddle` and `Ball` states (position, velocity, etc.), including temporary `Collided` flags. Handles player connections/disconnections (`AssignPlayerToRoom`, `PlayerDisconnect`) *initiated by ConnectionHandlerActor*. Sends initial state (`PlayerAssignmentMessage`, `InitialPlayersAndBallsState`) directly to the connecting client.
    -   **Physics Simulation (High Frequency):** Runs an internal `physicsTicker` (e.g., 60Hz). On each `GameTick`, it updates the positions of paddles and balls in its local cache (`updateInternalState`), performs collision detection using this cache (`detectCollisions`), updates scores/grid, handles power-ups. It generates **atomic update messages** (`BallPositionUpdate`, `PaddlePositionUpdate`, `ScoreUpdate`, `BallOwnershipChange`, `BallSpawned`, `BallRemoved`, `PlayerJoined`, `PlayerLeft`) reflecting any state changes during the tick and adds them to a pending update buffer. Position updates now include pre-calculated R3F coordinates.
    -   **State Broadcasting (Fixed Rate):** Runs an internal `broadcastTicker` (e.g., 30Hz). On each `BroadcastTick`, it generates a `FullGridUpdate` message containing a flat list of the state and pre-calculated R3F coordinates of **all** grid cells (`BrickStateUpdate`). It takes all pending atomic updates (including the grid update) from the buffer and sends them as a single batch (`BroadcastUpdatesCommand`) to its `BroadcasterActor`.
    -   **Game Logic:** Implements scoring rules, "hit own wall" logic (score penalty, lose ownership), "persistent ball" logic on player disconnect, and checks for game end condition (all bricks destroyed), triggering the game over sequence (stops tickers, sends `GameOverMessage`, notifies `RoomManagerActor`, stops self). Handles **phasing ball logic**: phasing balls pass through bricks without reflection, damaging each unique brick cell once per tick.
    -   **Cleanup:** Reliably stops internal tickers and child actors (`PaddleActor`, `BallActor`, `BroadcasterActor`) during its `Stopping` phase or upon panic recovery using `sync.Once` to prevent resource leaks and race conditions.
-   **BroadcasterActor**: Spawned by `GameActor`. Maintains the list of active WebSocket connections for its specific room (`AddClient`, `RemoveClient`). Receives batches of atomic updates (`BroadcastUpdatesCommand`) from its parent `GameActor`. Wraps the batch in a `GameUpdatesBatch` message, marshals it to JSON, and sends the payload to all connected clients in its room asynchronously. Handles client send errors and notifies the `GameActor` of disconnections detected during broadcast. Handles `GameOverMessage` by sending it to all clients and then closing their connections.
-   **Child Actors (PaddleActor, BallActor)**: Manage internal state based on commands received from their parent `GameActor`.
    -   `PaddleActor`: Updates its internal `Direction` based on `PaddleDirectionMessage`. Sends `PaddleStateUpdate` back to `GameActor` on change.
    -   `BallActor`: Updates its internal velocity, phasing status, mass, etc., based on commands like `SetVelocityCommand`, `ReflectVelocityCommand`, `SetPhasingCommand`. Sends `BallStateUpdate` back to `GameActor` on change.
    -   **Crucially, child actors DO NOT update their own positions based on a timer and DO NOT send position updates back to the `GameActor`.**
-   **State:** Game state is distributed. `RoomManagerActor` holds the list of rooms. Each `GameActor` holds the authoritative state for its room (players, grid, scores) and the **authoritative cache** of paddle/ball states used for simulation. `BroadcasterActor` holds client connections. Child actors manage their specific internal state (like velocity or direction) but not their absolute position over time. The client receives initial state messages (`PlayerAssignmentMessage`, `InitialPlayersAndBallsState`) and then reconstructs the current state by applying the received atomic updates from `GameUpdatesBatch` messages. All position data (paddles, balls, bricks) is now sent with pre-calculated R3F coordinates for direct rendering.
-   **Physics & Rules:** Collision detection (wall, brick, paddle) and response are handled within `GameActor` using its authoritative cache during the `GameTick`. Wall collisions immediately adjust the ball's position in the cache. Commands are sent to child actors to update their velocity/phasing for subsequent ticks. Rules for scoring, permanent balls, power-ups, and game completion are implemented here, using parameters from `utils/config.go`. Collision flags (`Collided`) are set in the `GameActor`'s cache during collision detection and included in the relevant atomic update messages (`BallPositionUpdate`, `PaddlePositionUpdate`). **Phasing balls** pass through bricks, damaging each unique cell once per tick without reflecting. Non-phasing balls reflect off bricks and trigger phasing. Brick life is determined by `GridBrickMinLife` and `GridBrickMaxLife` config values.
-   **Communication:** `GameActor` sends batches of atomic updates (`BroadcastUpdatesCommand`) or game over messages (`GameOverMessage`) to `BroadcasterActor`. `BroadcasterActor` sends JSON (`GameUpdatesBatch`, `GameOverMessage`) to clients. `GameActor` sends **commands** to children (`Send`). `ConnectionHandlerActor` interacts with `RoomManagerActor` (initially) and `GameActor` (directly). `RoomManagerActor` replies to `Ask` requests using `ctx.Reply()`. Child actors send internal state updates (`PaddleStateUpdate`, `BallStateUpdate`) back to `GameActor`.

## Key Components (Consolidated)

*   **room_manager.go**: Top-level coordinator (room lifecycle, assignment replies, list replies).
*   **game_actor.go**: Contains the `GameActor` struct definition, producer, and the main `Receive` message dispatch loop.
*   **game_actor_handlers.go**: Contains `GameActor` methods for handling specific messages (player connect/disconnect, input, ball commands).
*   **game_actor_physics.go**: Contains `GameActor` methods for physics simulation (`detectCollisions`) and related helpers.
*   **game_actor_state.go**: Contains `GameActor` methods for internal state updates (`updateInternalState`) and broadcasting (`handleBroadcastTick`).
*   **game_actor_lifecycle.go**: Contains `GameActor` methods for actor lifecycle management (start, stop, tickers, cleanup, game over).
*   **broadcaster_actor.go**: Handles asynchronous broadcasting of `GameUpdatesBatch` and `GameOverMessage` to clients within a room, manages connection cleanup.
*   **paddle_actor.go**: Manages internal paddle state (`Direction`) based on commands, sends `PaddleStateUpdate`.
*   **ball_actor.go**: Manages internal ball state (velocity, phasing, mass) based on commands, sends `BallStateUpdate`.
*   **game_actor_broadcast.go**: Contains `deepCopyGrid` helper (used only for initial grid send).
*   **paddle.go, ball.go, etc.**: Data structures (include `Collided` field). `Move()` methods are called by `GameActor` on cached objects.
*   **messages.go**: Defines all actor message types and atomic update message types (including R3F coordinates).
*   **grid.go, cell.go**: Grid and Cell data structures and grid generation logic (`FillSymmetrical`).

## Related Modules

*   [Bollywood Actor Library](https://github.com/lguibr/bollywood) (External Dependency)
*   [Server](../server/README.md) (Contains ConnectionHandlerActor)
*   [Utilities](../utils/README.md) (Contains config.go)
*   [Main Project](../README.md)