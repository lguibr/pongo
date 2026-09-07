# PonGo

A Go multiplayer Pong/Breakout game with up to four players per room. Players
control paddles on the arena edges, destroy bricks, collect power-ups, and score
by hitting opponents' walls. The game ends when the bricks are cleared.

## Backend architecture

- The room manager handles room codes, public Quick Play, slot reservations, and
  pending admissions.
- One game actor owns all simulation state in each room. Balls and paddles are
  ordinary state values; their movement and collision handling run in that room.
- Each connection has bounded input handling and an independent ordered writer.
  A slow client is disconnected without blocking other players.
- The broadcaster encodes each room batch once. Full grids are sent initially and
  on changes, rather than every broadcast.

Default simulation frequency is 40 Hz (25 ms). Broadcasting is capped at that
frequency. Idle lobbies emit a heartbeat every 15 seconds. Disconnected players
retain their slot for a 30-second reconnect grace period.

The room runtime lives in `internal/actor`; socket output lives in
`internal/transport`. The old external actor dependency and isolated entity actor
API remain declared for compatibility, but are not used by the running backend.

Read [the ownership and delivery decision](docs/adr/0001-room-state-and-delivery.md)
and [performance measurements](docs/performance.md) for details and tradeoffs.

## Run

```sh
go run .
```

The server listens on port 8080. Configuration is in `utils/config.go`.
The frontend is a separate project. This refactor preserves its JSON message
format and does not require frontend changes.

## Protocol

Connect to `/subscribe`, then send one of:

```json
{"messageType":"quickPlay","sessionId":"a-client-session-id"}
```

```json
{"messageType":"createRoom","isPublic":true,"sessionId":"a-client-session-id"}
```

```json
{"messageType":"joinRoom","code":"ABC123","sessionId":"a-client-session-id"}
```

Room creation/join replies precede player assignment and initial state. In a
lobby, send `{"messageType":"playerReady","isReady":true}`. Quick Play starts a
new room immediately after initialization. During play, send
`{"messageType":"direction","direction":"ArrowRight"}`; `ArrowLeft` and `Stop`
are also supported. Reuse the session ID and room code to reconnect.

`gameUpdates` batches carry position, score, ownership, lobby, and grid messages.
`fullGridUpdate` always replaces the entire grid. `gameOver` is the final message
before the server closes a healthy connection.

HTTP endpoints: `/rooms/` for room counts and `/health-check/` for liveness.

## Verification

```sh
go test -race ./game ./server ./internal/actor ./internal/transport
go test -race ./test -run '^TestE2E_(SinglePlayer|BallWall|StressTestGameCompletion)'
go test ./game -run '^$' -bench '^BenchmarkRoom' -benchmem -count 3
PONGO_CLIENTS=200 go test ./test -run '^TestPerformanceRooms$' -count=1 -v
PONGO_CLIENTS=40 PONGO_IDLE=1 go test ./test -run '^TestPerformanceRooms$' -count=1 -v
```

The performance workload joins four clients per room, reads continuously, sends
movement inputs, and verifies ongoing position delivery. It uses high brick life
and disables random power-ups to keep rooms alive for comparable measurements.
The completion workload uses empty arenas to test final delivery deterministically.

Several historical unit tests remain skipped; concrete lifecycle and socket
regressions now live in `game/lifecycle_regression_test.go` and
`server/lifecycle_test.go`. Local performance results are not a production
capacity or Internet-latency guarantee.
