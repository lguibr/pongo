
[![Coverage](https://img.shields.io/badge/Coverage-TBD-lightgrey)](./README.md) [![Unit-tests](https://img.shields.io/github/actions/workflow/status/lguibr/pongo/test.yml?label=UnitTests)](https://github.com/lguibr/pongo/actions/workflows/test.yml) [![Building](https://img.shields.io/github/actions/workflow/status/lguibr/pongo/build.yml?label=Build)](https://github.com/lguibr/pongo/actions/workflows/build.yml) [![Lint](https://img.shields.io/github/actions/workflow/status/lguibr/pongo/lint.yml?label=Lint)](https://github.com/lguibr/pongo/actions/workflows/lint.yml)


# PonGo
<p align="center">
  <img src="bitmap.png" alt="Logo" width="300"/>
</p>

# PonGo Game: Detailed Rules, Gameplay, and Architecture

This document details the workings of the PonGo game, a Pong/Breakout hybrid, based on the provided Go source code and configuration files. The game features a Go backend utilizing a custom actor model library (`Bollywood`) for concurrency and state management.

## 1. Introduction

PonGo combines elements of classic Pong (paddles deflecting a ball) with Breakout (breaking bricks with the ball). Players control paddles positioned on the edges of a square canvas, aiming to score points by breaking bricks in the center grid or by having the ball hit an opponent's wall after they were the last to touch it. The game supports up to 4 players simultaneously.

The backend is built using the Actor Model pattern via the internal `Bollywood` library. A central `GameActor` coordinates the game state and interactions between `PaddleActor`s (representing player paddles) and `BallActor`s (representing the balls in play).

## 2. Core Game Concepts

*   **Objective:** Score points by breaking bricks and hitting opponent walls. Keep your wall safe by bouncing balls away with your paddle. Avoid letting the ball hit your own wall after you were the last to touch it.
*   **Players:** Up to 4 players (`maxPlayers = 4`). Each player is assigned an index (0-3) corresponding to a side of the square arena:
    *   Player 0 → Right edge
    *   Player 1 → Top edge
    *   Player 2 → Left edge
    *   Player 3 → Bottom edge
*   **Game Area (`Canvas`):** A square area (`utils.CanvasSize`, default 576x576) containing a central grid of breakable bricks.
*   **Grid (`Grid`):** A 2D array (`utils.GridSize` x `utils.GridSize`, default 12x12) of `Cell`s within the canvas.
*   **Bricks (`Cell` with `BrickData`):** Cells in the grid can be `Empty` or `Brick`. Bricks have `Life` (hit points) and `Level` (score value). They are destroyed when their `Life` reaches 0. The initial grid layout is procedurally generated.
*   **Paddles (`Paddle`):** Rectangular objects controlled by players, positioned along their assigned edge of the canvas. Used to deflect balls.
*   **Balls (`Ball`):** Circular objects that move across the canvas, bouncing off walls, paddles, and bricks. Ownership tracks the last player to hit the ball.
*   **Scoring (Simplified Overview):** Points are gained for breaking bricks and hitting opponent walls. Points are lost if an opponent's ball hits your wall. (See "Score Mechanics" below for details).
*   **Power-ups:** Randomly triggered when breaking a brick, offering advantages like spawning extra balls, increasing ball mass/size, or increasing ball velocity. (See "Power-Ups" below for details).

## 3. Gameplay Rules & Mechanics

*(This section focuses on how the game plays from the user's perspective)*

### 3.1. Paddle Movement

-   Each paddle is oriented along one edge of the arena:
    -   Player 0 (Right) & Player 2 (Left) have **vertical** paddles moving **Up/Down**.
    -   Player 1 (Top) & Player 3 (Bottom) have **horizontal** paddles moving **Left/Right**.
-   Input commands (`ArrowLeft`, `ArrowRight`) are interpreted relative to the paddle's orientation:
    -   For **vertical** paddles (Index 0, 2):
        -   `"ArrowLeft"` (internal: `"left"`) means move **Up**.
        -   `"ArrowRight"` (internal: `"right"`) means move **Down**.
    -   For **horizontal** paddles (Index 1, 3):
        -   `"ArrowLeft"` (internal: `"left"`) means move **Left**.
        -   `"ArrowRight"` (internal: `"right"`) means move **Right**.
-   Paddles are constrained within the boundaries of their assigned edge and move at a fixed velocity.

### 3.2. Balls & Collisions

1.  **Spawning**: Each player gets at least one ball upon joining. More can be spawned via power-ups. Balls have varying velocity, radius, and mass.
2.  **Movement**: Balls move based on their `Vx`, `Vy` velocity vector, updated at regular intervals (ticks).
3.  **Wall Collision**:
    *   When a ball hits an edge of the canvas (a "wall"):
        *   Its velocity is reflected on the appropriate axis (X for side walls, Y for top/bottom walls).
        *   **Scoring:**
            *   If the hit wall belongs to a **different player** than the ball's current owner (`OwnerIndex`), the owner gains **+1 point**, and the player whose wall was hit loses **-1 point**.
            *   If the ball hits the wall belonging to its **own owner**, no score change occurs (logged as a "self-hit").
            *   If the wall belongs to an **unoccupied player slot**, the ball is removed from the game, and no points are awarded/deducted.
        *   The ball enters a brief `Phasing` state (see below).
4.  **Paddle Collision**:
    *   When a ball intersects with a paddle:
        *   Its velocity is reflected on the appropriate axis (X for vertical paddles, Y for horizontal paddles).
        *   The `OwnerIndex` of the ball is updated to the index of the player whose paddle was hit.
        *   The ball enters a brief `Phasing` state.
5.  **Brick Collision**:
    *   When a ball (that is *not* phasing) intersects a grid cell containing a `Brick`:
        *   The brick's `Life` is decremented.
        *   The ball's velocity is reflected based on the primary axis of impact (relative center positions).
        *   If the brick's `Life` reaches 0:
            *   The brick is destroyed (cell becomes `Empty`).
            *   The ball's `OwnerIndex` scores points equal to the brick's original `Level`.
            *   There is a chance (approx. 25%) to trigger a random Power-up.
        *   The ball enters a brief `Phasing` state.
6.  **Phasing**: A temporary state (lasting ~100ms) activated after wall, paddle, or brick collisions. While phasing, a ball ignores collisions with *bricks* (but can still collide with walls and paddles). This prevents immediate double-hits or getting stuck.

### 3.3. Power-Ups

Triggered randomly (approx. 25% chance) when a ball destroys a brick. The effect applies to the ball that broke the brick or spawns a new one for its owner:

1.  **SpawnBall**: Creates a new `BallActor` near the collision point, owned by the same player. This new ball might expire after a random short duration (5-9 seconds) or stay until it hits an empty wall.
2.  **IncreaseMass**: Increases the `Mass` of the ball and slightly increases its `Radius`. Heavier/larger balls have a bigger collision presence.
3.  **IncreaseVelocity**: Increases the ball's speed (`Vx`, `Vy`) by a factor (default 1.1), making it faster and potentially harder to react to.

### 3.4. Score Mechanics

*   Initial score: `utils.InitialScore` (default 100).
*   Score changes occur as follows:
    *   **+N points:** When your ball destroys a brick (where N is the brick's original `Level`/`Life`).
    *   **+1 point:** When your ball hits an *opponent's* wall.
    *   **-1 point:** When an *opponent's* ball hits *your* wall.
    *   **No change:** When your ball hits your *own* wall.
    *   **No change:** When any ball hits an unoccupied wall (the ball is just removed).

### 3.5. Ending Conditions

The current implementation does not have a built-in win/loss condition based on score or time. The game continues as long as players are connected. Potential end scenarios:

*   All players disconnect (server logs the game as inactive).
*   House rules (e.g., first to a certain score, last player remaining).

## 4. Controls

*(Based on typical frontend implementation)*

-   Use the **ArrowLeft** and **ArrowRight** keys to move your paddle.
-   The interpretation depends on your paddle's orientation (See section 3.1 Paddle Movement).
-   (Raw WebSocket message format: `{"direction": "ArrowLeft"}` or `{"direction": "ArrowRight"}`)

## 5. Tips & Strategy

-   **Manage Multiple Balls:** More balls mean more scoring opportunities but also more chaos to defend against.
-   **Utilize Power-ups:** Increased Mass makes your ball harder for others to handle; Increased Velocity can surprise opponents. Adapt your playstyle.
-   **Aim Intentionally:** Don't just react. Try to angle bounces off paddles or bricks to target opponent walls.
-   **Prioritize Brick Breaking:** Clears the field, grants points, and provides chances for powerful boosts.

## 6. FAQ (Frequently Asked Questions)

1.  **What if all bricks are broken?**
    The grid becomes empty. The game continues as a multi-directional Pong game until players decide to stop or disconnect.
2.  **What if a player disconnects mid-game?**
    Their paddle and any balls they owned are removed. Their score remains recorded (but inactive). The game continues with the remaining players.
3.  **Is there a formal “game over”?**
    Not currently implemented in the provided code. The game runs indefinitely as long as players are connected.
4.  **How do I see the scoreboard?**
    The game state broadcast over WebSocket includes score information for all connected players. A frontend client is needed to display this visually.

---

## 7. Technical Architecture (Actor Model)

*(This section provides a technical overview of the backend implementation)*

The game leverages the `Bollywood` actor library for managing concurrency and state.

### 7.1. Bollywood Actor Library Overview

*   **Engine:** Manages actor lifecycles (spawning, stopping) and message routing.
*   **Actor:** An interface (`Receive(Context)`) implemented by game entities (`GameActor`, `PaddleActor`, `BallActor`). Actors encapsulate state and behavior.
*   **PID (Process ID):** A unique identifier for addressing specific actor instances.
*   **Context:** Provided to `Receive`, allows actors to access their PID (`Self()`), the sender's PID (`Sender()`), the message (`Message()`), and the `Engine()`.
*   **Props:** Configuration used to spawn actors (contains a `Producer` function).
*   **Messages:** Arbitrary Go `interface{}` values passed between actors via the `Engine.Send` method. Actors process messages sequentially from their mailbox.
*   **System Messages:** `Started`, `Stopping`, `Stopped` are sent by the engine to actors during lifecycle events.

### 7.2. Key PonGo Actors

*   **`GameActor` (`game/game_actor.go`)**:
    *   **Role:** The central coordinator, orchestrator, and holder of the authoritative game state.
    *   **State:** Holds the `Canvas` (including the `Grid`), `players` (array of `*playerInfo`), `paddles` (live state array `[4]*Paddle`), `paddleActors` (PIDs array `[4]*bollywood.PID`), `balls` (live state map `map[int]*Ball`), `ballActors` (PID map `map[int]*bollywood.PID`), reference to the `Engine`, and cached `gameStateJSON`.
    *   **Responsibilities:** Manages player connections/disconnections, spawns/stops child actors, receives position updates, performs collision detection, resolves collisions by sending commands, manages scores, triggers power-ups, broadcasts game state.

*   **`PaddleActor` (`game/paddle_actor.go`)**:
    *   **Role:** Manages the state and movement logic for a single player's paddle.
    *   **State:** Holds a pointer to its `Paddle` data (`*Paddle`), a `time.Ticker`, and the PID of the `GameActor`.
    *   **Responsibilities:** Receives direction commands, updates position based on internal ticker and state, sends position updates back to `GameActor`.

*   **`BallActor` (`game/ball_actor.go`)**:
    *   **Role:** Manages the state and movement logic for a single ball.
    *   **State:** Holds a pointer to its `Ball` data (`*Ball`), a `time.Ticker`, the PID of the `GameActor`, and potentially a `phasingTimer`.
    *   **Responsibilities:** Updates position based on internal ticker and velocity, sends position updates back to `GameActor`, receives commands from `GameActor` to modify state (velocity, mass, phasing), manages phasing timer.

### 7.3. State Management

*   The *authoritative* game state resides within the `GameActor`.
*   `PaddleActor` and `BallActor` manage their *own* state and perform movement logic.
*   They report their updated state back to the `GameActor` via position messages.
*   The `GameActor` uses reported positions for collision detection and sends commands back to modify actor states.

### 7.4. Communication (Messages)

Key message types facilitate interaction: `PlayerConnectRequest`, `PlayerDisconnect`, `ForwardedPaddleDirection`, `PaddleDirectionMessage`, `PaddlePositionMessage`, `BallPositionMessage`, `ReflectVelocityCommand`, `SetPhasingCommand`, `IncreaseMassCommand`, `IncreaseVelocityCommand`, `DestroyBallCommand`, `SpawnBallCommand`, `GameStateUpdate`, `GameTick`, `internalTick`.

## 8. Gameplay Flow (Technical Step-by-Step)

*(This describes the sequence of events and messages within the actor system)*

1.  **Initialization:** Engine starts, spawns `GameActor`. `GameActor` starts its ticker.
2.  **Player Connection:** Client connects -> Server handler sends `PlayerConnectRequest` to `GameActor`.
3.  **GameActor Handles Connection:** Assigns index, creates `playerInfo`, spawns `PaddleActor` and `BallActor`, broadcasts state.
4.  **Server Assigns Index:** Server's `readLoop` polls until it finds the assigned index for its connection (indirectly via game state or internal map - needs clarification based on actual `AssignPlayerIndex` usage).
5.  **Player Input:** Client sends direction JSON -> Server `readLoop` forwards raw JSON in `ForwardedPaddleDirection` to `GameActor`.
6.  **GameActor Relays Input:** Forwards the raw JSON in `PaddleDirectionMessage` to the specific `PaddleActor`.
7.  **PaddleActor Processes Input:** Unmarshals JSON, updates internal `state.Direction`.
8.  **PaddleActor Moves:** `internalTick` -> `state.Move()` -> Sends `PaddlePositionMessage` to `GameActor`.
9.  **BallActor Moves:** `internalTick` -> `state.Move()` -> Sends `BallPositionMessage` to `GameActor`.
10. **GameActor Updates Internal State:** Receives position messages, updates its `paddles` and `balls` maps/arrays.
11. **GameActor Main Loop:** `GameTick` -> `detectCollisions()` -> Sends commands (`ReflectVelocityCommand`, etc.) to `BallActor`s -> Updates scores -> Potentially triggers power-ups (`SpawnBallCommand` to self, others to `BallActor`) -> Removes lost balls -> `broadcastGameState()` -> `updateGameStateJSON()`.
12. **BallActor Handles Commands:** Updates its `state` based on received commands (e.g., `state.ReflectVelocity()`).
13. **GameActor Broadcasts:** Marshals `GameState` to JSON -> Sends JSON via `Write` method of each active `playerInfo.Ws`. Handles write errors by triggering disconnect.
14. **Player Disconnection:** Triggered by read error/EOF or write error -> `Server.CloseConnection` cleans up server state -> `readLoop` defer sends `PlayerDisconnect` to `GameActor` -> `GameActor` stops associated actors, cleans up its state, broadcasts update.

## 9. Game Entities & State Details

*(Data structures holding game object information)*

*   **`Canvas` (`game/canvas.go`)**: Contains `Grid`, dimensions (`Width`, `Height`, `CanvasSize`), `GridSize`, `CellSize`.
*   **`Grid` (`game/grid.go`)**: `[][]Cell`. Provides methods for procedural generation (`Fill`).
*   **`Cell` (`game/cell.go`)**: Holds `X`, `Y` grid coordinates and `*BrickData`.
*   **`BrickData` (`game/cell.go`)**: Contains `Type` (Brick/Empty), `Life` (hits left), `Level` (score value).
*   **`Paddle` (`game/paddle.go`)**: Holds `X`, `Y`, `Width`, `Height`, `Index`, internal `Direction` ("left"/"right"/""), `Velocity`, `canvasSize`. Contains `Move()` logic.
*   **`Ball` (`game/ball.go`)**: Holds `X`, `Y`, `Vx`, `Vy`, `Radius`, `Id`, `OwnerIndex`, `Phasing` status, `Mass`, `canvasSize`. Contains `Move()` logic and state modification methods (`ReflectVelocity`, etc.).
*   **`Player` (`game/player.go`)**: Simplified struct for JSON state (`Index`, `Id`, `Color`, `Score`).
*   **`playerInfo` (`game/game_actor.go`)**: Internal `GameActor` struct for live player state (`Index`, `ID`, `Score`, `Color`, `Ws` connection interface, `IsConnected` flag).

## 10. Key Game Parameters

*(Default values from `utils/constants.go`)*

*   `Period`: 24ms (Tick rate).
*   `InitialScore`: 100.
*   `CanvasSize`: 576.
*   `GridSize`: 12.
*   `CellSize`: 48.
*   `BallMass`: 1.
*   `BallSize`: 12 (`CellSize / 4`).
*   `PaddleLength`: 144 (`CellSize * 3`).
*   `PaddleWeight`: 24 (`CellSize / 2`).

## 11. Server & Networking

*   Standard Go `net/http` server on port `3001`.
*   `/subscribe`: WebSocket endpoint for game connections.
*   `/`: HTTP GET endpoint (intended for JSON game state, currently placeholder).
*   `Server` struct (`server/websocket.go`) manages connections and holds `Engine`/`GameActor` references.

## 12. Building and Running

*   **Requires:** Go (1.19+).
*   **Build:** `go build` in project root.
*   **Run:** `go run main.go` or execute the compiled binary.
*   **Connect:** Use WebSocket client to `ws://localhost:3001/subscribe`.

## 13. Development & CI

*   Uses Go modules.
*   GitHub Actions for Build, Lint, Test (includes coverage reporting and badge update).
*   `.gitignore` excludes standard files, vendor, frontend.
*   `Dockerfile` for containerization.