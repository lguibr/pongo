# File: README.md

[![Coverage](https://img.shields.io/badge/Coverage-70%25+-yellow)](./README.md) [![Unit-tests](https://img.shields.io/github/actions/workflow/status/lguibr/pongo/test.yml?label=UnitTests)](https://github.com/lguibr/pongo/actions/workflows/test.yml) [![Building](https://img.shields.io/github/actions/workflow/status/lguibr/pongo/build.yml?label=Build)](https://github.com/lguibr/pongo/actions/workflows/build.yml) [![Lint](https://img.shields.io/github/actions/workflow/status/lguibr/pongo/lint.yml?label=Lint)](https://github.com/lguibr/pongo/actions/workflows/lint.yml)

# PonGo

<p align="center">
  <img src="bitmap.png" alt="Logo" width="300"/>
</p>

# PonGo Game: Detailed Rules, Gameplay, and Architecture

This document details the workings of the PonGo game, a Pong/Breakout hybrid, based on the provided Go source code and configuration files. The game features a Go backend utilizing a custom actor model library (`Bollywood`) for concurrency and state management. Key game parameters are now configurable in `utils/config.go`.

*... (rest of the README, update gameplay rules for permanent ball and paddle stop)* ...*

### 3.1. Paddle Movement

-   Each paddle is oriented along one edge of the arena:
    -   Player 0 (Right) & Player 2 (Left) have **vertical** paddles moving **Up/Down**.
    -   Player 1 (Top) & Player 3 (Bottom) have **horizontal** paddles moving **Left/Right**.
-   Input commands (`ArrowLeft`, `ArrowRight`, `Stop`) are interpreted relative to the paddle's orientation:
    -   For **vertical** paddles (Index 0, 2):
        -   `"ArrowLeft"` (internal: `"left"`) means move **Up**.
        -   `"ArrowRight"` (internal: `"right"`) means move **Down**.
    -   For **horizontal** paddles (Index 1, 3):
        -   `"ArrowLeft"` (internal: `"left"`) means move **Left**.
        -   `"ArrowRight"` (internal: `"right"`) means move **Right**.
    -   `"Stop"` (internal: `""`) means **stop moving**.
-   Paddles are constrained within the boundaries of their assigned edge and move at a fixed velocity defined in the config. Releasing keys sends a "Stop" command.

### 3.2. Balls & Collisions

1.  **Spawning**: Each player gets one **permanent** ball upon joining. This ball is never removed for hitting an empty wall or expiring. More temporary balls can be spawned via power-ups. Balls have varying velocity, radius, and mass. Initial balls spawn with a **random velocity vector**.
2.  **Movement**: Balls move based on their `Vx`, `Vy` velocity vector, updated at regular intervals (ticks).
3.  **Wall Collision**:
    *   When a ball hits an edge of the canvas (a "wall"):
        *   Its velocity is reflected on the appropriate axis.
        *   **Scoring:** (Same as before)
        *   **Ball Removal:** If the wall belongs to an **unoccupied player slot**, the ball is removed *only if it is not a permanent ball*. Permanent balls are reflected instead.
        *   The ball enters a brief `Phasing` state.

*... (rest of the README)* ...*

## 10. Key Game Parameters

*(Default values now sourced from `utils/config.go` - see `DefaultConfig()`)*

*   `GameTickPeriod`: 24ms
*   `InitialScore`: 100
*   `CanvasSize`: 576
*   `GridSize`: 12
*   `CellSize`: 48
*   `BallMass`: 1
*   `BallRadius`: 12
*   `PaddleLength`: 144
*   `PaddleWidth`: 24
*   `PaddleVelocity`: 8 (Example, check config)
*   `MinBallVelocity`: ~2.88
*   `MaxBallVelocity`: ~3.84
*   `PowerUpChance`: 0.25 (25%)
*   *(See `utils/config.go` for all parameters)*

*... (rest of the README)* ...*

## 14. Submodules

*   [Game Logic](./game/README.md)
*   [Server](./server/README.md)
*   [Bollywood Actor Library](./bollywood/README.md)
*   [Utilities](./utils/README.md) (Contains `config.go`)
*   [Frontend](./frontend/README.md)