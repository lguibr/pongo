# File: pongo/game/README.md

# Game Logic Module

This module contains the core gameplay logic, state management, and actor implementations for the PonGo game, built using the [Bollywood Actor Library](https://github.com/lguibr/bollywood).

## Overview

The game logic is orchestrated by actors. A central RoomManagerActor manages multiple GameActor instances. A temporary ConnectionHandlerActor (in the Server module) manages each WebSocket connection. Each GameActor spawns child actors for game entities and a dedicated BroadcasterActor for state dissemination.

-   **RoomManagerActor**: Manages the lifecycle of GameActor instances. Handles requests (FindRoomRequest) from ConnectionHandlerActor to find or create rooms. Cleans up empty rooms (GameRoomEmpty). Responds to HTTP queries (GetRoomListRequest via Ask/Reply).
-   **ConnectionHandlerActor (in Server module)**: Manages a single WebSocket connection. Asks RoomManagerActor for a room assignment. Once assigned, communicates *directly* with the designated GameActor to assign the player, forward input, and signal disconnection.
-   **GameActor**: Represents a single game room (up to 4 players). Manages the core game state (canvas, grid, players, scores). Spawns and supervises child actors (PaddleActor, BallActor) and a BroadcasterActor. Drives child actor updates via UpdatePositionCommand. Queries child actor state (GetPositionRequest) for collision detection using Engine.Ask. Performs all collision detection and physics calculations. Updates scores and grid state. Handles power-up logic. Implements the "persistent ball" logic on player disconnect. Periodically creates a GameState snapshot and sends it (BroadcastStateCommand) to its BroadcasterActor. Notifies the RoomManagerActor when it becomes empty (GameRoomEmpty).
-   **BroadcasterActor**: Spawned by GameActor. Maintains the list of active WebSocket connections for its specific room (AddClient, RemoveClient). Receives GameState snapshots (BroadcastStateCommand) from its parent GameActor. Marshals the state to JSON. Sends the JSON payload to all connected clients in its room asynchronously. Handles client send errors and notifies the GameActor of disconnections detected during broadcast.
-   **Child Actors (PaddleActor, BallActor)**: Manage individual game entities (paddles, balls). Update their state upon receiving UpdatePositionCommand. Respond to GetPositionRequest (via ctx.Reply()) with their current state. Handle direct commands (e.g., SetVelocityCommand, PaddleDirectionMessage) from their parent GameActor.
-   **State:** Game state is distributed. RoomManagerActor holds the list of rooms and approximate player counts. Each GameActor holds the authoritative state for its room (players, grid, scores) and caches the last known state of paddles/balls (updated via Ask). BroadcasterActor holds client connections for the room. Child actors manage their local state. ConnectionHandlerActor holds the WebSocket connection reference.
-   **Physics & Rules:** Collision detection (wall, brick, paddle) and response are handled within GameActor after querying child positions via Ask. Rules for scoring, permanent balls, and power-ups are implemented here, using parameters from utils/config.go.
-   **Communication:** Actors communicate via messages defined in messages.go. GameActor sends state snapshots to BroadcasterActor. BroadcasterActor sends JSON to clients. GameActor commands children (Send) and queries them (Ask). ConnectionHandlerActor interacts with RoomManagerActor (initially) and GameActor (directly). RoomManagerActor and child actors reply to Ask requests using ctx.Reply().

## Key Components

*   **room_manager.go**: Top-level coordinator (room lifecycle, assignment replies, list replies).
*   **game_actor.go**: Single game room coordinator (physics ticks, state management, child supervision, broadcaster management).
*   **broadcaster_actor.go**: Handles asynchronous broadcasting of game state to clients within a room.
*   **paddle_actor.go**: Manages paddle state and movement based on commands/queries.
*   **ball_actor.go**: Manages ball state and movement based on commands/queries.
*   **game_actor_physics.go**: Collision detection logic (queries child states via Ask).
*   **game_actor_handlers.go**: Handlers for GameActor messages (connect/disconnect from ConnectionHandler).
*   **game_actor_broadcast.go**: Contains createGameStateSnapshot helper function.
*   **paddle.go, ball.go, etc.**: Data structures.
*   **messages.go**: Defines all actor message types.

## Related Modules

*   [Bollywood Actor Library](https://github.com/lguibr/bollywood) (External Dependency)
*   [Server](../server/README.md) (Contains ConnectionHandlerActor)
*   [Utilities](../utils/README.md) (Contains config.go)
*   [Main Project](../README.md)
