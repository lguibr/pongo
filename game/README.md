
# Game Logic Module

This module contains the core gameplay logic, state management, and actor implementations for the PonGo game, built using the [Bollywood Actor Library](https://github.com/lguibr/bollywood). It features a decoupled architecture where high-frequency physics simulation is separated from fixed-rate network broadcasting of atomic state changes.

## Overview

The game logic is orchestrated by actors. A central `RoomManagerActor` manages multiple `GameActor` instances. A temporary `ConnectionHandlerActor` (in the Server module) manages each WebSocket connection. Each `GameActor` spawns child actors for game entities (`PaddleActor`, `BallActor`) and a dedicated `BroadcasterActor` for state dissemination.

-   **RoomManagerActor**: Manages the lifecycle of `GameActor` instances.
-   **ConnectionHandlerActor (in Server module)**: Manages a single WebSocket connection.
-   **GameActor**: Represents a single game room. Manages core game state and the authoritative local cache of entity states.
    -   **Physics Simulation (High Frequency):** Runs an internal `physicsTicker`. On each `GameTick`:
        1.  `moveEntities()`: Updates positions of paddles and balls in its local cache based on their current velocities from the *previous* tick.
        2.  `detectCollisions()`: Performs collision detection using the updated cache. Resolves collisions by updating cached entity velocities (e.g., reflecting ball velocity). **Crucially, direct position adjustments (snapping) are avoided; only velocities are changed to ensure continuous movement.** Sends commands to child actors to update their internal states. Handles scoring and power-ups. Generates atomic event updates.
        3.  `generatePositionUpdates()`: Creates `BallPositionUpdate` and `PaddlePositionUpdate` messages from the final cached state of the current tick.
        4.  `resetPerTickCollisionFlags()`: Resets `Collided` flags in the cache for the next tick.
        5.  Checks for game end condition.
    -   **State Broadcasting (Fixed Rate):** Runs an internal `broadcastTicker`. On each `BroadcastTick`, it generates a `FullGridUpdate` and sends all pending atomic updates as a batch to its `BroadcasterActor`.
    -   **Game Logic:** Implements scoring, "hit own wall" logic, "persistent ball" logic, power-ups, and game over conditions.
    -   **Phasing Ball Logic:** Phasing balls pass through bricks but reflect normally off walls/paddles without resetting their phasing timer or triggering scoring (for wall hits).
    -   **Cleanup:** Reliably stops internal tickers and child actors.
-   **BroadcasterActor**: Manages WebSocket connections for its room and broadcasts game state updates.
-   **Child Actors (PaddleActor, BallActor)**: Manage internal state (direction, velocity, phasing) based on commands from `GameActor`. They **do not** calculate their own positions.
-   **State:** Game state is distributed. `GameActor` holds the authoritative state for its room and entity caches. Clients reconstruct state from atomic updates.
-   **Physics & Rules:** Collision detection and response (velocity reflection) are handled within `GameActor`. Ball positions are *not* forcibly adjusted on collision.
-   **Communication:** Actors communicate via messages. `GameActor` sends batched updates to `BroadcasterActor`, which sends JSON to clients.

## Key Components (Consolidated)

*   **room_manager.go**: Top-level coordinator.
*   **game_actor.go**: `GameActor` struct, producer, `Receive` loop.
*   **game_actor_handlers.go**: `GameActor` message handlers.
*   **game_actor_physics.go**: `GameActor` physics simulation (`detectCollisions`, collision handlers, power-ups).
*   **game_actor_state.go**: `GameActor` internal state updates (`moveEntities`, `generatePositionUpdates`, `resetPerTickCollisionFlags`, `handleBroadcastTick`, R3F mapping).
*   **game_actor_lifecycle.go**: `GameActor` lifecycle management.
*   **broadcaster_actor.go**: Broadcasting logic.
*   **paddle_actor.go, ball_actor.go**: Entity actor logic.
*   **paddle.go, ball.go, etc.**: Data structures. `Move()` methods update cached objects based on velocity.
*   **messages.go**: Actor and atomic update message definitions.
*   **grid.go, cell.go**: Grid/Cell structures and generation.
*   **collision_tracker.go**: Tracks ongoing collisions.

## Related Modules

*   [Bollywood Actor Library](https://github.com/lguibr/bollywood)
*   [Server](../server/README.md)
*   [Utilities](../utils/README.md)
*   [Main Project](../README.md)
