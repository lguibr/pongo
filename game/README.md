# File: game/README.md
# Game Logic Module

This module contains the core gameplay logic, state management, and actor implementations for the PonGo game, built using the [Bollywood Actor Library](../bollywood/README.md).

## Core Actors

*   **`GameActor` (`game_actor.go`)**:
    *   The central coordinator, holding references to game state and child actor PIDs.
    *   Manages the game lifecycle and main game tick.
    *   Dispatches incoming messages to handler methods located in `game_actor_handlers.go`.
    *   Initiates collision detection handled in `game_actor_physics.go`.
    *   Triggers game state broadcasting handled in `game_actor_broadcast.go`.

*   **`PaddleActor` (`paddle_actor.go`)**:
    *   Manages the state and movement logic for a single paddle (`Paddle` struct).
    *   Receives `PaddleDirectionMessage`.
    *   Updates position based on direction and an internal ticker.
    *   Sends `PaddlePositionMessage` updates to the `GameActor`.

*   **`BallActor` (`ball_actor.go`)**:
    *   Manages the state and movement logic for a single ball (`Ball` struct).
    *   Updates position based on velocity and an internal ticker.
    *   Sends `BallPositionMessage` updates to the `GameActor`.
    *   Receives commands from the `GameActor` to modify its state.
    *   Manages its own phasing state.

## Supporting Files

*   **`game_actor_handlers.go`**: Contains handler functions for messages received by `GameActor` (e.g., player connect/disconnect, position updates, input forwarding).
*   **`game_actor_physics.go`**: Contains the collision detection logic (`detectCollisions`, `findCollidingCells`) and power-up triggering logic run during the game tick.
*   **`game_actor_broadcast.go`**: Contains the logic for marshalling the current game state (`GameState` struct) and broadcasting it to connected clients.

## State Structs

*   **`Paddle` (`paddle.go`)**: Data structure holding paddle state.
*   **`Ball` (`ball.go`)**: Data structure holding ball state.
*   **`Canvas` (`canvas.go`)**: Represents the game area, including the `Grid`.
*   **`Grid` (`grid.go`)**: A 2D array of `Cell`s representing the breakable bricks. Includes grid generation logic.
*   **`Cell` (`cell.go`)**: Represents a single cell in the grid, containing `BrickData`.
*   **`Player` (`player.go`)**: Data structure holding player-specific info for JSON state representation.

## Message Flow (Simplified)

1.  **Input:** `WebSocket Handler` -> `ForwardedPaddleDirection` (with WsConn) -> `GameActor` -> `handlePaddleDirection` -> `PaddleDirectionMessage` -> `PaddleActor`.
2.  **Paddle Update:** `PaddleActor` (on tick) -> `Move()` -> `PaddlePositionMessage` -> `GameActor` -> `handlePaddlePositionUpdate`.
3.  **Ball Update:** `BallActor` (on tick) -> `Move()` -> `BallPositionMessage` -> `GameActor` -> `handleBallPositionUpdate`.
4.  **Game Logic:** `GameActor` (on tick) -> `detectCollisions()` -> Sends commands (e.g., `ReflectVelocityCommand`) -> `BallActor`.
5.  **State Broadcast:** `GameActor` (on tick) -> `broadcastGameState()` -> Sends to all connected `PlayerConnection`s -> Clients.

## Related Modules

*   [Bollywood Actor Library](../bollywood/README.md)
*   [Server](../server/README.md)
*   [Main Project](../README.md)