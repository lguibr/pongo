# PonGo
![Coverage](https://img.shields.io/badge/Coverage-62.0%25-yellow)

A multiplayer Pong/Breakout game for up to four players per room. Each player guards one
edge of a square arena with a paddle, breaks bricks, collects power-ups and scores by
hitting opponents' walls. The game ends when the last brick is destroyed.

This repository is the Go game server. The browser client lives in the separate
`pongo-client` repository; the two share only the WebSocket protocol described below.

## Gameplay

### Joining

- **Quick Play** joins the public room with space that is furthest from starting (lobby
  before countdown before play). If there is none, it creates a room that starts at once.
- **Create room** makes a public or private room with a six-character code; **join room**
  enters one by code.
- A room holds four players. A joining player gets the average score of the connected
  players (the configured start score in an empty room), a paddle and one permanent ball.
- A player who disconnects keeps their slot, paddle and balls for 30 seconds.
  Reconnecting with the same session id and room code resumes them.

### Paddles

| Player | Edge | `ArrowLeft` | `ArrowRight` |
| --- | --- | --- | --- |
| 0 | right | up | down |
| 1 | top | left | right |
| 2 | left | up | down |
| 3 | bottom | left | right |

`Stop` halts the paddle. Directions are relative to the arena; the client rotates each
player's view and translates on-screen keys to these directions.

### Balls

- Each player's **permanent ball** spawns near their paddle and stays in the game.
  **Temporary balls** come from the spawn-ball power-up and expire after about 12 seconds.
- Balls reflect off walls, paddles and bricks by changing velocity; positions are never
  snapped.
- **Phasing** (a power-up lasting 3 seconds) lets a ball pass through bricks, damaging each
  brick it touches once. A phasing ball still reflects off walls and paddles, and its wall
  hits do not score.

### Scoring

- A non-phasing ball hitting a connected player's wall costs that player a point and gives
  the ball's owner a point. A player who hits their own wall only loses the point, and the
  ball becomes ownerless.
- A non-phasing ball hitting the wall of an empty slot reflects if it is permanent and is
  removed if it is temporary.
- Hitting a paddle makes that player the ball's owner. Where the ball meets the paddle sets
  the rebound angle, and the paddle's movement changes its speed.
- A brick loses one life per hit. Destroying it gives the ball's owner points equal to the
  brick's level and, 40% of the time, a power-up: spawn a ball, grow the ball, speed it up,
  or start phasing.

### End of game

When the last brick is destroyed, the connected player with the highest score wins (a tie
has no winner) and every client receives `gameOver`. When a disconnected player's grace
expires, their permanent balls become ownerless and their temporary balls are removed. A
room with no connected players closes after another 30 seconds.

## Architecture

- `RoomManagerActor` handles room codes, Quick Play, slot reservations and pending
  admissions.
- One `GameActor` per room owns all simulation state and runs physics at 40 Hz. Balls and
  paddles are plain values inside it, and only its goroutine touches them.
- Each connection has a handler actor with bounded input and its own ordered writer. A slow
  client is disconnected without affecting the others.
- The broadcaster encodes each batch once. The full grid is sent when a player joins and
  after a brick changes; idle rooms send a heartbeat every 15 seconds.

The runtime is `internal/actor`: reliable in-process queues with bounded ingress for
network input. Socket output is `internal/transport`. See the [game](game/README.md) and
[server](server/README.md) packages, [ADR 0001](docs/adr/0001-room-state-and-delivery.md),
[ADR 0002](docs/adr/0002-remove-compatibility-layer.md) and the
[performance measurements](docs/performance.md).

## Run

Requires Go 1.24.

```sh
go run .
```

The server listens on `PORT` (default 8080). `PONGO_LOG_LEVEL` selects `debug`, `info`,
`warn` or `error`; the default is `info`. With Docker:

```sh
docker build -t pongo .
docker run -p 8080:8080 pongo
```

## Configuration

Defaults come from `DefaultConfig()` in `utils/config.go`:

| Setting | Default |
| --- | --- |
| Physics tick | 25 ms (40 Hz) |
| Broadcast rate | 40 Hz, never faster than physics |
| Canvas, grid, cell | 900 px, 18 × 18, 50 px |
| Ball speed, radius | 5–10 px per tick, 8 px |
| Paddle length, width, speed | 150 px, 25 px, 12 px per tick |
| Brick fill, life | 55% of the free area, 1–7 hits |
| Clear zones | radius 1 around the centre, 3 cells from each wall |
| Power-up chance | 40% per destroyed brick |
| Spawned ball lifetime | 12 s ± 2 s |
| Phasing | 3 s |
| Mass power-up, speed power-up | +2 mass (radius +2 per mass), ×1.09 |

Limits: 4 players per room, 75 rooms, 512 concurrent connections.

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

The room reply (`roomCreated` or `roomJoined`) comes before `playerAssignment`, the initial
entities and the first full grid. In a lobby, send
`{"messageType":"playerReady","isReady":true}`; when every connected player is ready a
3-second countdown starts. During play, send
`{"messageType":"direction","direction":"ArrowRight"}`; `ArrowLeft` and `Stop` are also
accepted. Reuse the session id and room code to reconnect.

`gameUpdates` batches carry position, score, ownership, lobby and grid messages.
`fullGridUpdate` always replaces the whole grid. `gameOver` is the last message before the
server closes a healthy connection.

A session id is required (at most 128 characters). A room request must arrive within 10
seconds of connecting. Frames over 4 KiB or more than 60 messages per second close the
connection.

HTTP: `/rooms/` returns room ids with player counts; `/` and `/health-check/` report
liveness.

## Verification

```sh
go build ./... && go vet ./... && golangci-lint run ./...
go test -race ./game ./server ./internal/actor ./internal/transport ./utils
go test -race ./test -run '^TestE2E_(SinglePlayer|BallWall|StressTestGameCompletion)'
go test ./game -run '^$' -bench '^BenchmarkRoom' -benchmem -count 3
PONGO_CLIENTS=200 go test ./test -run '^TestPerformanceRooms$' -count=1 -v
```

The performance workload joins four clients per room, reads continuously, sends movement
input and checks that positions keep arriving. It uses long-lived bricks and no power-ups
so runs are comparable; see [test/README.md](test/README.md). Local results are not a
production capacity or latency guarantee.

GitHub Actions runs golangci-lint (configured in `.golangci.yml`), the race-enabled test
suite with coverage, and a build that pushes the Docker image on `main` and version tags.
