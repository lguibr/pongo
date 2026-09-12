# End-to-end tests

These tests start the real HTTP and WebSocket server in-process and talk to it with the
client protocol.

| Test | What it checks |
| --- | --- |
| `TestE2E_SinglePlayerConnectMoveStopDisconnect` | Admission, initial state, paddle movement start and stop, disconnect |
| `TestE2E_BallWallNonStick` | Balls move away from a wall after hitting it |
| `TestE2E_NonPhasingBallBrickPenetration` | Non-phasing ball centres are never reported inside bricks (skipped with `-short`) |
| `TestE2E_StressTestGameCompletion` | 40 rooms with empty arenas finish and every client receives `gameOver` (skipped with `-short`) |
| `TestE2E_StressTestMultipleRooms` | The performance workload with 200 clients (skipped with `-short`) |
| `TestPerformanceRooms` | Four clients per room read continuously and send input; position delivery is asserted |

## Running

```sh
go test -race ./test -run '^TestE2E_(SinglePlayer|BallWall|StressTestGameCompletion)'
PONGO_CLIENTS=200 go test ./test -run '^TestPerformanceRooms$' -count=1 -v
PONGO_CLIENTS=40 PONGO_IDLE=1 go test ./test -run '^TestPerformanceRooms$' -count=1 -v
```

`PONGO_CLIENTS` sets the number of clients (default 4). `PONGO_IDLE=1` keeps the rooms in
the lobby. Take timing measurements without `-race`, one run at a time, on an otherwise
idle machine; the figures include the simulated clients running in the same process.

## Profiling

```sh
PONGO_CLIENTS=200 go test ./test -run '^TestPerformanceRooms$' -count=1 \
  -cpuprofile /tmp/pongo-cpu.prof -memprofile /tmp/pongo-mem.prof -o /tmp/pongo-test-binary
go tool pprof -top /tmp/pongo-cpu.prof
```
