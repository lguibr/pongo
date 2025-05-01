[![Coverage](https://img.shields.io/badge/Coverage-TBD%25-lightgrey)](./README.md) [![Unit-tests](https://img.shields.io/github/actions/workflow/status/lguibr/pongo/test.yml?label=UnitTests)](https://github.com/lguibr/pongo/actions/workflows/test.yml) [![Build & Push](https://img.shields.io/github/actions/workflow/status/lguibr/pongo/build.yml?label=Build%20%26%20Push)](https://github.com/lguibr/pongo/actions/workflows/build.yml) [![Lint](https://img.shields.io/github/actions/workflow/status/lguibr/pongo/lint.yml?label=Lint)](https://github.com/lguibr/pongo/actions/workflows/lint.yml) [![Docker Image](https://img.shields.io/docker/pulls/lguibr/pongo.svg?label=Docker%20Pulls)](https://hub.docker.com/r/lguibr/pongo) <!-- Replace lguibr with your Docker Hub username -->

# PonGo: Multi-Room Pong/Breakout Hybrid

<p align="center">
  <img src="bitmap.png" alt="Logo" width="300"/>
</p>

Welcome to PonGo, a real-time multiplayer game combining elements of Pong and Breakout. This project features a Go backend built with a custom actor model library ([Bollywood](https://github.com/lguibr/bollywood)) designed for concurrency and scalability, supporting multiple independent game rooms.

## Table of Contents

- [PonGo: Multi-Room Pong/Breakout Hybrid](#pongo-multi-room-pongbreakout-hybrid)
  - [Table of Contents](#table-of-contents)
  - [1. Overview](#1-overview)
  - [2. Gameplay Rules](#2-gameplay-rules)
    - [2.1 Objective](#21-objective)
    - [2.2 Joining a Game](#22-joining-a-game)
    - [2.3 Paddle Control](#23-paddle-control)
    - [2.4 Balls](#24-balls)
    - [2.5 Collisions \& Scoring](#25-collisions--scoring)
    - [2.6 Bricks \& Power-ups](#26-bricks--power-ups)
    - [2.7 Winning/Losing](#27-winninglosing)
  - [3. Architecture](#3-architecture)
    - [3.1 Actor Model (Bollywood)](#31-actor-model-bollywood)
    - [3.2 Connection Handling (`ConnectionHandlerActor`)](#32-connection-handling-connectionhandleractor)
    - [3.3 Room Management (`RoomManagerActor`)](#33-room-management-roommanageractor)
    - [3.4 Game Room (`GameActor`)](#34-game-room-gameactor)
    - [3.5 Broadcasting (`BroadcasterActor`)](#35-broadcasting-broadcasteractor)
    - [3.6 Entity Actors (`PaddleActor`, `BallActor`)](#36-entity-actors-paddleactor-ballactor)
    - [3.7 Communication Flow (Diagram)](#37-communication-flow-diagram)
  - [4. Key Game Parameters](#4-key-game-parameters)
  - [5. Setup \& Running](#5-setup--running)
    - [5.1 Prerequisites](#51-prerequisites)
    - [5.2 Backend](#52-backend)
    - [5.3 Frontend](#53-frontend)
    - [5.4 Docker](#54-docker)
      - [5.4.1 Building Locally](#541-building-locally)
      - [5.4.2 Running Pre-built Image (Docker Hub)](#542-running-pre-built-image-docker-hub)
  - [6. Testing](#6-testing)
  - [7. API Endpoints](#7-api-endpoints)
  - [8. Submodules](#8-submodules)
  - [9. Contributing](#9-contributing)

## 1. Overview

PonGo pits up to four players against each other in a square arena filled with destructible bricks. Each player controls a paddle on one edge of the arena, defending their side and attempting to score points by hitting opponents' walls or destroying bricks. The game utilizes WebSockets for real-time communication and Go's concurrency features managed by the Bollywood actor library to handle game state and player interactions efficiently across multiple game rooms.

## 2. Gameplay Rules

### 2.1 Objective

The primary goal is to achieve the highest score by hitting opponent walls, destroying bricks, and outlasting other players. Players lose points when a ball hits their assigned wall.

### 2.2 Joining a Game

-   Players connect via WebSocket to the server.
-   The server's **Room Manager** assigns the player to the first available game room (up to 4 players per room).
-   If all existing rooms are full, the Room Manager automatically creates a new room for the player.
-   Upon joining, the player is assigned an index (0-3), a paddle, a color, an initial score, and one **permanent ball**.

### 2.3 Paddle Control

-   Each player controls a paddle fixed to one edge:
    -   Player 0 (Right Edge): Vertical Paddle (Moves Up/Down)
    -   Player 1 (Top Edge): Horizontal Paddle (Moves Left/Right)
    -   Player 2 (Left Edge): Vertical Paddle (Moves Up/Down)
    -   Player 3 (Bottom Edge): Horizontal Paddle (Moves Left/Right)
-   Input commands (`ArrowLeft`, `ArrowRight`, `Stop`) control paddle movement *relative to its orientation*:
    -   **Vertical Paddles (0 & 2):**
        -   `ArrowLeft` -> Move **Up**
        -   `ArrowRight` -> Move **Down**
    -   **Horizontal Paddles (1 & 3):**
        -   `ArrowLeft` -> Move **Left**
        -   `ArrowRight` -> Move **Right**
    -   `Stop` (or releasing movement keys) -> **Stop** movement immediately.
-   Paddles are confined to their assigned edge and move at a configured velocity (`PaddleVelocity`).

### 2.4 Balls

1.  **Permanent Ball:** Each player receives one **permanent ball** upon joining. This ball is associated with the player but is never removed from the game if it hits an empty wall (it reflects instead). Its ownership might change if another player hits it.
2.  **Temporary Balls:** Additional balls can be spawned through power-ups. These balls *are* removed if they hit a wall belonging to an empty player slot. They also expire after a randomized duration.
3.  **Initial Spawn:** Permanent balls spawn near their owner's paddle with a randomized initial velocity vector.
4.  **Movement:** Balls move according to their velocity vector (`Vx`, `Vy`), updated each game tick.
5.  **Ownerless Ball:** If the last player in a room disconnects, one of their balls (preferably permanent) will be kept in play, marked as ownerless (`OwnerIndex = -1`) and permanent, ensuring the game always has at least one ball if players remain.

### 2.5 Collisions & Scoring

1.  **Wall Collision:**
    *   **Reflection:** The ball's velocity component perpendicular to the wall is reversed (command sent to `BallActor`).
    *   **Active Player Wall:** If the wall belongs to a connected player (the "conceder"):
        *   The conceder loses 1 point.
        *   The player who last hit the ball (the "scorer", if different from the conceder and still connected) gains 1 point.
        *   Ownerless balls hitting an active player's wall cause the wall owner to lose 1 point.
    *   **Empty Player Slot Wall:**
        *   If the ball is **permanent**, it reflects as normal (no scoring).
        *   If the ball is **temporary**, it is removed from the game (`GameActor` stops the `BallActor`).
    *   **Phasing:** After any wall collision, the ball enters a brief "phasing" state (command sent to `BallActor`).

2.  **Paddle Collision:**
    *   **Dynamic Reflection:** The ball reflects off the paddle. The reflection angle depends on *where* the ball hits the paddle surface.
    *   **Speed Influence:** The paddle's current velocity component *along* the ball's reflection path influences the ball's resulting speed.
    *   **Ownership:** The player whose paddle was hit becomes the new owner of the ball (state updated in `GameActor`'s cache).
    *   **Phasing:** The ball enters the phasing state (command sent to `BallActor`).

3.  **Brick Collision:**
    *   **Damage:** The brick's `Life` decreases by 1 (state updated in `GameActor`).
    *   **Reflection:** The ball reflects off the brick surface (command sent to `BallActor`).
    *   **Destruction:** If `Life` reaches 0:
        *   The brick is removed (`Type` becomes `Empty`).
        *   The ball's current owner (if valid and connected) gains points equal to the brick's initial `Level`.
        *   There's a chance (`PowerUpChance`) to trigger a random power-up.
    *   **Phasing:** The ball enters the phasing state (command sent to `BallActor`). Bricks cannot be hit by phasing balls.

### 2.6 Bricks & Power-ups

-   **Bricks:** Occupy cells in the central grid. They have `Life` (hit points) and `Level` (points awarded on destruction). The grid is procedurally generated when a room is created.
-   **Power-ups:** Triggered randomly when a brick is destroyed. Effects apply to the ball that broke the brick or spawn new entities:
    -   **Spawn Ball:** Creates a new temporary ball near the broken brick, owned by the player who broke the brick (`GameActor` spawns a new `BallActor`).
    -   **Increase Mass:** Increases the mass and radius of the ball that broke the brick (command sent to `BallActor`).
    -   **Increase Velocity:** Increases the speed of the ball that broke the brick (command sent to `BallActor`).

### 2.7 Winning/Losing

-   The game continues as long as players are connected. There isn't an explicit win condition defined by score in the current rules, but players aim to maximize their score.
-   Players effectively "lose" if they disconnect.
-   If all players disconnect, the room becomes empty and is eventually cleaned up by the Room Manager.

## 3. Architecture

PonGo uses an Actor Model architecture facilitated by the [Bollywood](https://github.com/lguibr/bollywood) library. This promotes concurrency and isolates state management.

### 3.1 Actor Model (Bollywood)

-   Actors are independent units of computation with private state.
-   They communicate solely through asynchronous messages (`Send`) or synchronous request/reply (`Ask`).
-   The `Engine` manages actor lifecycles (spawning, stopping) and message routing.
-   Actors use the `Context` provided in `Receive` to interact, including `ctx.Reply()` for `Ask` responses.

### 3.2 Connection Handling (`ConnectionHandlerActor`)

-   A dedicated, short-lived actor spawned by the server for each new WebSocket connection.
-   **Responsibilities:**
    -   Asks the `RoomManagerActor` for a game room assignment (`FindRoomRequest`).
    -   Receives the assigned `GameActor` PID (`AssignRoomResponse`).
    -   Sends `AssignPlayerToRoom` *directly* to the assigned `GameActor`.
    -   Manages the `readLoop` for the WebSocket connection.
    -   Forwards player input (`ForwardedPaddleDirection`) *directly* to the assigned `GameActor`.
    -   Sends `PlayerDisconnect` *directly* to the assigned `GameActor` upon connection error or closure.
    -   Stops itself when the connection terminates.

### 3.3 Room Management (`RoomManagerActor`)

-   A central actor managing the list of active game rooms.
-   **Responsibilities:**
    -   Handles `FindRoomRequest` from `ConnectionHandlerActor`.
    -   Finds an existing `GameActor` (room) with space or spawns a new one (up to a limit).
    -   Replies to `ConnectionHandlerActor` with the assigned `GameActor` PID (`AssignRoomResponse`).
    -   Receives notifications (`GameRoomEmpty`) from `GameActors` when they become empty.
    -   Stops empty `GameActors` and removes them from the active list.
    -   Handles requests for the list of active rooms (`GetRoomListRequest` from HTTP handler via `Ask`) and replies using `ctx.Reply()`.
    -   **Does NOT directly interact with WebSockets or handle player input.**

### 3.4 Game Room (`GameActor`)

-   Each instance represents a single, independent game room (max 4 players).
-   **Responsibilities:**
    -   Manages the core state of a specific game: Canvas, Grid, Players, Scores.
    -   Maintains local caches of `Paddle` and `Ball` states.
    -   Handles player connections/disconnections (`AssignPlayerToRoom`, `PlayerDisconnect`) *initiated by ConnectionHandlerActor*.
    -   Spawns and supervises child actors (`PaddleActor`, `BallActor`) and a `BroadcasterActor`.
    -   Drives child actor updates via `UpdatePositionCommand`.
    -   Receives `PositionUpdateMessage` from child actors and updates its local state cache.
    -   Performs all collision detection and physics calculations using its cached state.
    -   Sends commands (`SetVelocity`, `ReflectVelocity`, `SetPhasing`, etc.) to child actors based on collision results.
    -   Updates scores and grid state.
    -   Handles power-up logic (spawning new balls, sending commands to existing balls).
    -   Implements the "persistent ball" logic on player disconnect.
    -   Periodically creates a `GameState` snapshot and sends it (`BroadcastStateCommand`) to its `BroadcasterActor`.
    -   Notifies the `RoomManagerActor` when it becomes empty (`GameRoomEmpty`).

### 3.5 Broadcasting (`BroadcasterActor`)

-   A dedicated actor spawned by each `GameActor`.
-   **Responsibilities:**
    -   Maintains the list of active WebSocket connections for its specific room (`AddClient`, `RemoveClient`).
    -   Receives `GameState` snapshots (`BroadcastStateCommand`) from its parent `GameActor`.
    -   Marshals the state to JSON.
    -   Sends the JSON payload to all connected clients in its room.
    -   Handles WebSocket write errors and notifies the `GameActor` of disconnections detected during broadcast.

### 3.6 Entity Actors (`PaddleActor`, `BallActor`)

-   **`PaddleActor`:** Manages the state (position, velocity, direction) of a single paddle. Updates state on `UpdatePositionCommand`. Sends `PositionUpdateMessage` to `GameActor` after update. Handles `PaddleDirectionMessage` from `GameActor`. Responds to `GetPositionRequest` (via `ctx.Reply()`) if needed.
-   **`BallActor`:** Manages the state (position, velocity, phasing) of a single ball. Updates state on `UpdatePositionCommand`. Sends `PositionUpdateMessage` to `GameActor` after update. Handles commands (`SetVelocity`, `ReflectVelocity`, `SetPhasing`, etc.) from `GameActor`. Responds to `GetPositionRequest` (via `ctx.Reply()`) if needed.

### 3.7 Communication Flow (Diagram)

```mermaid
sequenceDiagram
    participant Client
    participant ServerHandler
    participant ConnectionHandlerActor
    participant RoomManagerActor
    participant GameActor
    participant BroadcasterActor
    participant PaddleActor
    participant BallActor

    Client->>+ServerHandler: WebSocket Connect (/subscribe)
    ServerHandler->>+ConnectionHandlerActor: Spawn(WsConn, Engine, RoomManagerPID)
    ConnectionHandlerActor->>+RoomManagerActor: FindRoomRequest {ReplyTo: SelfPID}
    alt Room Found/Created
        RoomManagerActor-->>-ConnectionHandlerActor: AssignRoomResponse {RoomPID}
        ConnectionHandlerActor->>+GameActor: AssignPlayerToRoom {WsConn}
        GameActor->>+BroadcasterActor: AddClient {WsConn}
        GameActor->>+PaddleActor: Spawn PaddleActor
        GameActor->>+BallActor: Spawn BallActor (Permanent)
        %% ConnectionHandlerActor starts readLoop (internal)
    else No Room Available / Error
        RoomManagerActor-->>-ConnectionHandlerActor: AssignRoomResponse {RoomPID: nil}
        ConnectionHandlerActor->>ConnectionHandlerActor: Cleanup (Close WsConn, Stop Self)
    end
    ServerHandler-->>-Client: (Connection stays open if successful)


    loop Game Loop (GameActor Physics Tick)
        GameActor->>GameActor: GameTick (Internal Timer)
        GameActor->>PaddleActor: UpdatePositionCommand
        GameActor->>BallActor: UpdatePositionCommand
        %% Children update internally and send state back
        PaddleActor-->>GameActor: PositionUpdateMessage {State}
        BallActor-->>GameActor: PositionUpdateMessage {State}
        %% GameActor updates its internal cache upon receiving PositionUpdateMessage

        GameActor->>GameActor: detectCollisions()
        Note over GameActor: Uses internal state cache for detection
        opt Collision Detected
            GameActor->>BallActor: ReflectVelocityCommand / SetVelocityCommand / etc.
            GameActor->>GameActor: Update Score / Grid
            opt PowerUp Triggered
                 GameActor->>GameActor: SpawnBallCommand / IncreaseMassCommand / etc.
            end
        end
    end

    loop Broadcast Loop (GameActor Broadcast Tick)
        GameActor->>GameActor: BroadcastTick (Internal Timer)
        GameActor->>GameActor: createGameStateSnapshot()
        GameActor->>+BroadcasterActor: BroadcastStateCommand {State}
        BroadcasterActor->>BroadcasterActor: Marshal State to JSON
        BroadcasterActor->>Client: Send GameState JSON (to all clients in room)
        opt Send Error
            BroadcasterActor->>BroadcasterActor: Mark client disconnected
            BroadcasterActor->>GameActor: PlayerDisconnect {WsConn}
        end
    end


    Client->>+ConnectionHandlerActor: Send Input (e.g., {"direction":"ArrowLeft"}) (via readLoop)
    ConnectionHandlerActor->>+GameActor: ForwardedPaddleDirection {WsConn, Data}
    GameActor->>PaddleActor: PaddleDirectionMessage {Data}


    Client->>-ConnectionHandlerActor: WebSocket Disconnect (detected by readLoop)
    ConnectionHandlerActor->>+GameActor: PlayerDisconnect {WsConn}
    GameActor->>GameActor: Handle Disconnect (Stop Actors, Persistent Ball Logic, Clean Cache)
    GameActor->>+BroadcasterActor: RemoveClient {WsConn}
    opt Last Player Left
        GameActor->>+RoomManagerActor: GameRoomEmpty {RoomPID}
        RoomManagerActor->>GameActor: Stop Actor (via Engine)
    end
    ConnectionHandlerActor->>ConnectionHandlerActor: Stop Self

    %% HTTP Request for Room List
    participant HTTPClient
    HTTPClient->>+ServerHandler: GET /rooms/
    ServerHandler->>+RoomManagerActor: Ask(GetRoomListRequest)
    RoomManagerActor-->>-ServerHandler: Reply(RoomListResponse)
    ServerHandler-->>-HTTPClient: JSON Response

```

## 4. Key Game Parameters

All major game parameters are configurable in `utils/config.go`. See the `DefaultConfig()` function for default values. Key parameters include:

-   `GameTickPeriod`: (Default: 24ms)
-   `CanvasSize`, `GridSize`, `CellSize`
-   `InitialScore`
-   `PaddleLength`, `PaddleWidth`, `PaddleVelocity`
-   `MinBallVelocity`, `MaxBallVelocity`, `BallRadius`, `BallMass`, `BallPhasingTime`
-   Paddle/Ball collision physics factors (`BallHitPaddleSpeedFactor`, `BallHitPaddleAngleFactor`)
-   Grid generation parameters
-   Power-up chances and parameters (`PowerUpChance`, `PowerUpSpawnBallExpiry`, etc.)

## 5. Setup & Running

### 5.1 Prerequisites

-   Go (version 1.19 or later recommended)
-   Git
-   Docker (Optional, for containerized deployment)
-   Node.js/npm (For running the frontend)

### 5.2 Backend

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/lguibr/pongo.git
    cd pongo
    ```
2.  **Fetch dependencies:**
    ```bash
    go mod tidy
    ```
3.  **Run the server:**
    ```bash
    go run main.go
    ```
    The backend server will start, typically on `http://localhost:8080`.

### 5.3 Frontend

1.  **Navigate to the frontend directory:**
    ```bash
    cd frontend
    ```
2.  **Install dependencies:**
    ```bash
    npm install
    ```
3.  **Start the development server:**
    ```bash
    npm run dev
    ```
    The frontend will usually be available at `http://localhost:5173` (or similar, check console output). Open this URL in your browser.

### 5.4 Docker

#### 5.4.1 Building Locally

1.  **Build the backend image:**
    ```bash
    docker build -t pongo-backend .
    ```
2.  **Run the backend container:**
    ```bash
    docker run -p 8080:8080 pongo-backend
    ```
    (Ensure the frontend is configured to connect to the backend at the correct address if running separately).


#### 5.4.2 Running Pre-built Image (Docker Hub)

A pre-built image is automatically pushed to Docker Hub from the `main` branch. Replace `lguibr` with the correct Docker Hub username if it differs.

1.  **Pull the latest image:**
    ```bash
    docker pull lguibr/pongo:latest
    ```
2.  **Run the container (mapping port 8080):**
    ```bash
    # Map host port 8080 to container port 8080
    docker run -d -p 8080:8080 --name pongo-server lguibr/pongo:latest
    ```
    This runs the container in detached mode (`-d`) and maps port 8080. The frontend should be configured to connect to `ws://<your-docker-host-ip>:8080/subscribe`.


## 6. Testing

-   **Unit Tests:** Run standard Go tests.
    ```bash
    go test ./...
    ```
-   **Linting:** Uses `golangci-lint`. Ensure it's installed or run via CI.
    ```bash
    golangci-lint run ./...
    ```
-   **End-to-End (E2E) Tests:** Located in the `test/` directory. These simulate client connections and interactions.
    ```bash
    go test ./test -v -run E2E
    ```
-   **Coverage:** Generate coverage reports.
    ```bash
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out
    ```

## 7. API Endpoints

-   **`ws://<host>:8080/subscribe`**: The primary WebSocket endpoint for game clients to connect.
-   **`http://<host>:8080/rooms/`**: HTTP GET endpoint. Returns a JSON object listing active game rooms (by PID string) and their current player counts (e.g., `{"actor-1": 2, "actor-3": 4}`).
-   **`http://<host>:8080/`**: HTTP GET endpoint for health check. Returns `{"status": "ok"}`.
-   **`http://<host>:8080/health-check/`**: Explicit health check endpoint. Returns `{"status": "ok"}`.

## 8. Submodules

*   [Game Logic](./game/README.md): Core gameplay, actor implementations (GameActor, PaddleActor, BallActor), Room Manager.
*   [Server](./server/README.md): HTTP/WebSocket connection handling, interaction with Room Manager.
*   [Bollywood Actor Library](./bollywood/README.md): External actor library dependency.
*   [Utilities](./utils/README.md): Configuration (`config.go`), constants, helper functions.
*   [Frontend](./frontend/README.md): Svelte frontend application.

## 9. Contributing

Contributions are welcome! Please follow standard Go practices, ensure tests pass, and update documentation as needed. Open an issue to discuss major changes.

