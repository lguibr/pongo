
```markdown
# File: game/README.md
# Game Logic Module

This module contains the core gameplay logic, state management, and actor implementations for the PonGo game, built using the [Bollywood Actor Library](../bollywood/README.md).

## Core Actors

*   **`GameActor` (`game_actor.go`)**:
    *   The central coordinator and holder of the authoritative game state (`Canvas`, player info, paddle/ball states, actor PIDs).
    *   Manages the game lifecycle: player connections/disconnections, spawning/stopping child actors.
    *   Receives position updates (`PaddlePositionMessage`, `BallPositionMessage`) from child actors.
    *   Performs collision detection (ball-wall, ball-paddle, ball-brick).
    *   Sends commands (`ReflectVelocityCommand`, `SetPhasingCommand`, etc.) to `BallActor`s based on collisions.
    *   Manages player scores and triggers power-ups.
    *   Receives forwarded player input (`ForwardedPaddleDirection`) and relays it to the appropriate `PaddleActor`.
    *   Broadcasts the overall `GameState` to connected clients (via handlers/actors managing WebSockets).

*   **`PaddleActor` (`paddle_actor.go`)**:
    *   Manages the state and movement logic for a single paddle (`Paddle` struct).
    *   Receives `PaddleDirectionMessage` from the `GameActor`.
    *   Updates its position based on direction and an internal ticker (`internalTick`).
    *   Sends `PaddlePositionMessage` updates to the `GameActor`.

*   **`BallActor` (`ball_actor.go`)**:
    *   Manages the state and movement logic for a single ball (`Ball` struct).
    *   Updates its position based on velocity and an internal ticker (`internalTick`).
    *   Sends `BallPositionMessage` updates to the `GameActor`.
    *   Receives commands from the `GameActor` to modify its state (e.g., `ReflectVelocityCommand`, `SetPhasingCommand`, `IncreaseVelocityCommand`, `IncreaseMassCommand`, `DestroyBallCommand`).
    *   Manages its own phasing state via an internal timer (`SetPhasingCommand`, `stopPhasingCommand`).

## State Structs

*   **`Paddle` (`paddle.go`)**: Data structure holding paddle state (position, size, direction, etc.).
*   **`Ball` (`ball.go`)**: Data structure holding ball state (position, velocity, radius, owner, etc.).
*   **`Canvas` (`canvas.go`)**: Represents the game area, including the `Grid`.
*   **`Grid` (`grid.go`)**: A 2D array of `Cell`s representing the breakable bricks.
*   **`Cell` (`cell.go`)**: Represents a single cell in the grid, containing `BrickData`.
*   **`Player` (`player.go`)**: Data structure holding player-specific info (ID, score, color) primarily for JSON state representation. Live connection state is managed elsewhere (e.g., `GameActor.playerInfo`).

## Message Flow (Simplified)

1.  **Input:** `WebSocket Handler` -> `ForwardedPaddleDirection` -> `GameActor` -> `PaddleDirectionMessage` -> `PaddleActor`.
2.  **Paddle Update:** `PaddleActor` (on tick) -> `Move()` -> `PaddlePositionMessage` -> `GameActor`.
3.  **Ball Update:** `BallActor` (on tick) -> `Move()` -> `BallPositionMessage` -> `GameActor`.
4.  **Game Logic:** `GameActor` (on tick) -> `detectCollisions()` -> Sends commands (e.g., `ReflectVelocityCommand`) -> `BallActor`.
5.  **State Broadcast:** `GameActor` (on tick) -> Marshals `GameState` -> Sends to all `WebSocket Handlers` -> Clients.

## Related Modules

*   [Bollywood Actor Library](../bollywood/README.md)
*   [Server](../server/README.md)
*   [Main Project](../README.md)