# File: game/README.md
# Game Logic Module

This module contains the core gameplay logic, state management, and actor implementations for the PonGo game, built using the [Bollywood Actor Library](https://github.com/lguibr/bollywood).

## Overview

The core of the game logic resides in the `GameActor`, which orchestrates the interactions between players, paddles, balls, and the game grid. It manages the game state, handles connections, detects collisions, calculates scores, and broadcasts updates to connected clients. Game parameters like speed, sizes, and timings are now centralized in `utils/config.go`.

-   **Actors:** `GameActor`, `PaddleActor`, `BallActor`.
-   **State:** Game state (canvas, grid, players, paddles, balls) is primarily held by `GameActor`. Child actors manage their own local state (position, velocity) and report updates.
-   **Physics:** Collision detection and response are handled in `GameActor`.
    -   Wall collisions result in simple reflections and potential scoring. Balls hitting walls of empty player slots are removed, *unless* it's a player's initial, permanent ball.
    -   Brick collisions result in reflections, brick damage/destruction, scoring, and potential power-up triggers.
    -   Paddle collisions use dynamic physics: the reflection angle depends on the hit location on the paddle, and the paddle's current velocity influences the ball's resulting speed and direction.
-   **Ball Spawning:** Each player receives one **permanent** ball upon joining, which is never removed for hitting an empty wall or expiring. Power-ups can spawn additional, temporary balls. Initial balls spawn with a random velocity vector.
-   **Paddle Movement:** Paddles move based on "ArrowLeft" / "ArrowRight" commands relative to their orientation. Sending a "Stop" command (or releasing keys) halts paddle movement.
-   **Communication:** Actors communicate via messages defined in `messages.go`. `GameActor` broadcasts the overall `GameState` to clients via WebSocket.

## Key Components

*   **`game_actor.go`**: The central coordinator. Manages game lifecycle, player connections, state aggregation, collision detection, scoring, and broadcasting. Uses configuration from `utils/config.go`.
*   **`paddle_actor.go`**: Manages a single paddle's movement logic based on input messages ("left", "right", or "" for stop). Updates its internal state (including `Vx`, `Vy`) and sends `PaddlePositionMessage` to `GameActor`.
*   **`ball_actor.go`**: Manages a single ball's movement logic. Updates its position based on velocity and sends `BallPositionMessage` to `GameActor`. Receives commands (`SetVelocityCommand`, `ReflectVelocityCommand`, etc.) from `GameActor` to modify its state. Manages phasing timer.
*   **`game_actor_physics.go`**: Contains the `detectCollisions` logic, including wall, brick, and dynamic paddle collision handling. Handles permanent ball logic for wall hits. Also includes power-up triggering logic based on config.
*   **`game_actor_handlers.go`**: Contains handlers for specific messages received by `GameActor` (e.g., player connect/disconnect, position updates). Handles spawning permanent vs temporary balls.
*   **`game_actor_broadcast.go`**: Handles marshalling the `GameState` and sending it to connected clients.
*   **`paddle.go`, `ball.go`, `cell.go`, `grid.go`, `canvas.go`, `player.go`**: Define the data structures for game entities and provide associated methods. `Paddle` now stops moving when `Direction` is empty. `Ball` now includes an `IsPermanent` flag. `NewBall` initializes with random velocity and `IsPermanent` status. Uses config values for initialization.
*   **`messages.go`**: Defines the message types used for actor communication. `SpawnBallCommand` includes `IsPermanent`.

## Related Modules

*   [Bollywood Actor Library](https://github.com/lguibr/bollywood) (External Dependency)
*   [Server](../server/README.md)
*   [Utilities](../utils/README.md) (Contains `config.go`)
*   [Main Project](../README.md)