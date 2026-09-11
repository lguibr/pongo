# Game package

Room logic for PonGo: the room manager, one game actor per room, the broadcaster and the
game data types.

## Ownership

`RoomManagerActor` (`room_manager.go`) keeps room codes, public Quick Play rooms, slot
reservations and pending admissions. It creates a `GameActor` per room. A slot is
released only when an admission is rejected or a player's reconnect grace expires.

`GameActor` owns everything in its room: players, paddles, balls, the brick grid, scores
and phase. Only its own goroutine touches that state, so none of it is behind a lock.
Tickers and timers never read the actor; they send it messages, and timer messages carry
a generation number so a stale expiry is ignored.

`BroadcasterActor` (`broadcaster_actor.go`) encodes each batch of updates once and hands
the same bytes to every client's writer (`internal/transport`). A client whose output
queue overflows is disconnected; the room never waits on a socket.

## Tick loop

Physics runs every `GameTickPeriod` (25 ms by default):

1. `moveEntities` moves paddles and balls by their velocity.
2. `detectCollisions` reflects velocities off walls, paddles and bricks and applies
   scoring, brick damage and power-ups. Positions are never snapped.
3. `generatePositionUpdates` queues position messages.
4. `resetPerTickCollisionFlags` clears the one-tick collision flags.
5. `checkGameOver` ends the game when no bricks remain.

Broadcasting runs at `BroadcastRateHz` and never faster than physics. The full grid is
sent when a player joins and after a brick changes; an idle room sends an empty batch
every 15 seconds. Ticks are coalesced, so a room that falls behind skips ticks instead
of queueing them.

## Players

- **Admission** (`game_actor_admission.go`): validate the request, pick a slot (a
  returning session gets the slot held for it), give a new player the average score of
  the connected players, fill the grid for the first player, send the assignment and
  initial state, then announce the player.
- **Disconnect** (`game_actor_disconnect.go`): the slot, paddle and balls are held for
  30 seconds. When the grace expires, the player's permanent balls become ownerless, the
  temporary ones are removed and the room manager frees the slot. A room left with no
  connected players closes after another 30 seconds.
- **Lobby** (`game_actor_lobby.go`): readiness, the 3-second countdown and entering play.
  Quick Play rooms start as soon as the grid exists.

## Files

| File | Holds |
| --- | --- |
| `room_manager.go` | Room codes, reservations and admission tracking |
| `game_actor.go` | `GameActor` state, constructor and message dispatch |
| `game_actor_admission.go` | Admission and joining |
| `game_actor_disconnect.go` | Disconnect, reconnect grace and empty-room close |
| `game_actor_lobby.go` | Readiness, countdown and phase changes |
| `game_actor_entities.go` | Paddle input, ball spawn and expiry, end of phasing |
| `game_actor_physics.go` | Collisions, scoring, bricks, power-ups and phasing timers |
| `game_actor_state.go` | Update batching and the full-grid snapshot |
| `game_actor_lifecycle.go` | Start, tickers, cleanup, game over and metrics |
| `broadcaster_actor.go` | Encode once and fan out to client writers |
| `messages.go` | Wire messages and internal actor messages |
| `ball.go`, `paddle.go`, `player.go` | Entity data and movement |
| `grid.go`, `cell.go`, `canvas.go` | Brick grid generation and the canvas |
| `collision_tracker.go` | Ongoing ball–paddle and ball–brick contacts |
| `test_utils.go` | Helpers shared with the end-to-end tests |

See [the server package](../server/README.md), [ADR 0001](../docs/adr/0001-room-state-and-delivery.md)
and [ADR 0002](../docs/adr/0002-remove-compatibility-layer.md).
