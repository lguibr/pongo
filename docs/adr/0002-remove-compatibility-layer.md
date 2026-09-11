# Remove the compatibility layer around the room runtime

Date: 2026-09-11
Status: implemented on refactor/room-performance; supersedes the compatibility clause of ADR 0001

## Context

ADR 0001 moved room state into the game actor and replaced the external actor library
with `internal/actor`, but kept the old entity actor types, their messages, the external
`bollywood` module in `go.mod` and a partially tracked `vendor/` directory for source
compatibility. Nothing in the running server used them. Locks written for the old design
stayed on state that only one goroutine touched, logging went to stdout through `fmt`,
and the module still targeted Go 1.19.

## Decision

- Remove `BallActor`, `PaddleActor`, their messages and PID bookkeeping, the tests that
  exercised them, and the `bollywood` and `uuid` requirements. `vendor/` is no longer
  tracked; builds use the module cache.
- Import the runtime as `actor`.
- Game and room-manager state is touched only by the owning actor's goroutine, so the
  mutexes and atomics on it are removed, the `CollisionTracker` lock included. Ticker
  and timer callbacks receive the engine, the PID and, for tickers, the atomic
  `PendingTick` flag when they are created, and never read actor fields.
- Log through `log/slog`: debug for room chatter, info for lifecycle events and per-room
  metrics, warn and error for failures. `PONGO_LOG_LEVEL` selects the level. The runtime
  logs recovered actor panics with their stack.
- Target Go 1.24. The server listens on `PORT`, defaulting to 8080.
- Split the room handlers into admission, disconnect, entity and lobby files, with unit
  tests for slot choice, joining score, lobby state and the reconnect grace period.

## Consequences

- New room behaviour is a method on `GameActor` or a message to it; there is no second
  copy of entity state to keep in step.
- Anything that runs off the actor's goroutine, such as a timer, communicates by message.
  The race detector is the check for this rule.
- Internal queues remain unbounded. Each room's metrics line reports its largest backlog
  (`maxQueue`) so growth is visible.
- The external actor library is not a dependency and should not be reintroduced.
