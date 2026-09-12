# Backend performance check

Date: 2026-09-07 (America/Sao_Paulo)
Branch: refactor/room-performance
Baseline: main, 75548d5b9f1abec76670f862a63bc4d56de12de8
Host: Apple M3 Pro, macOS arm64, Go 1.24.2

Verified Go-source fingerprint (sorted relative paths and file bytes, SHA-256):
`8b27c6043a81c1f5b66355444b0182e1e2736cae3913001e63a008753e70a5ef`.
Local raw logs, retained baseline binary, and profiles from this run are in
`/tmp/pongo-performance-20260907/`; `verified-200.log` and `verified-200.cpu`
correspond to the final 200-client row. These scratch artifacts may be temporary;
the test sources and measurement summary are included in the branch.

## Method

The original 200-client stress test did not send the current admission handshake,
did not continuously consume output, and counted workers that failed to connect
as successful. Its historical metrics are not a valid baseline.

The replacement test uses the real HTTP/WebSocket server and protocol. Clients
create/join rooms in groups of four, assert assignment, continuously read frames,
ready the room, and send inputs every 100 ms. After four seconds for admission and
countdown, it samples five seconds. Every active run checks ongoing position
updates and zero unexpected read failures. Grid life is 100000 and power-ups are
disabled to prevent random game completion during measurement. Grid placement and
initial ball direction remain random; collision-dependent traffic varies between
runs. These are local comparative measurements, not a statistical capacity study.

The baseline was compiled before edits and retained separately. The original
source was also restored from Git into a temporary checkout for identical
microbenchmarks and the idle workload. No application source changes were needed
to measure the baseline.

Race detection is enabled for correctness tests, not for timing measurements.
Allocation, heap, goroutine, and CPU figures include both the server and simulated
clients in the same process. They must not be described as server-only resource
usage. CPU profiles cover the approximately 9.2-second whole run; throughput and
allocation rates cover its five-second sampling window. Position-gap percentiles
cover received position batches during the active portion of the run.

## Active rooms

| Clients / rooms | Baseline kB/client/s | Refactor kB/client/s | Baseline position frames/s | Refactor position frames/s | Baseline p95 gap | Refactor p95 gap |
|---|---:|---:|---:|---:|---:|---:|
| 4 / 1 | 785.4 | 67.8 | 40.00 | 40.00 | 34.32 ms | 25.76 ms |
| 40 / 10 | 784.5 | 85.9 | 39.98 | 39.99 | 33.73 ms | 25.71 ms |
| 200 / 50 | 787.3 | 87.8 | 40.03 | 39.97 | 33.54 ms | 25.33 ms |

Decimal kB are used. All runs admitted every requested client and had zero
unexpected reader failures. At 200 clients, outgoing bytes per client decreased
by about 89%, while useful position delivery remained approximately 40 Hz.
The 40 Hz simulation interval is unchanged.

At 200 clients, combined-process allocation volume fell from 1110.3 MB/s to
118.6 MB/s (89% lower), and allocations from 407268/s to 224623/s. The end-of-sample
goroutine count fell from 1405 to 1205. Live heap snapshots were 30.1 MB and
10.4 MB respectively, but heap snapshots depend on GC timing and are not peak
memory measurements. Sampled CPU time fell from 15.82 seconds to 4.83 seconds
across equivalent approximately 9.2-second profiles (69% lower). Profiles show
socket/runtime work dominating rather than collision math.

## Idle lobbies

With 40 clients and no ready messages, the baseline emitted 60 frames/s and
739.8 kB/client/s. The refactor emitted zero frames during the same five-second
sampling window. It still sends a small heartbeat every 15 seconds, so sustained
idle traffic is not literally zero. Inputs are still sent by this test; combined
allocation volume fell from 214.1 MB/s to 0.84 MB/s.

## Room microbenchmarks

Three samples per benchmark, using the same fixtures in both source versions:

| Benchmark | Baseline median | Refactor median | Baseline bytes/op | Refactor bytes/op |
|---|---:|---:|---:|---:|
| Stable-grid broadcast preparation | 1469 ns | 319.2 ns | 11800 | 856 |
| Four-ball physics/update generation | 860.3 ns | 879.3 ns | approximately 705 | approximately 705 |

Stable-grid preparation is about 4.6 times faster and allocates 93% fewer bytes.
Physics is essentially unchanged (a small 2% difference in these samples); this
refactor does not claim faster collision math. Broadcast preparation excludes
encoding and socket IO; those are exercised by the multiroom workload.

## Correctness and adverse conditions

Targeted race-enabled checks cover:

- More than 1024 internal messages delivered in order, and bounded ingress rejection.
- Concurrent Stop/Shutdown, spawn versus shutdown, and panic after an Ask reply.
- Slow writes, overflow while a socket writer is blocked, and final-message order.
- Room stop before admission dequeue, duplicate pending sessions, and exactly-once
  slot release after a committed connection fails.
- Room cleanup before broadcaster registration, stale power-up expiry, and Quick
  Play waiting for grid initialization.
- Reconnect slot preservation, fifth-player rejection, repeated room requests,
  oversized input, and pending-admission timeout while input continues.
- Forty concurrent game completions, final delivery, and return to only the room
  manager actor.
- Existing movement/stop, wall-reflection, brick-collision, and phasing tests.

The full race-enabled suite passed: `go test -race ./... -count=1 -timeout 120s`.
All packages passed, including the repaired 200-client workload; the end-to-end
package completed in 45.4 seconds on this host. Several old skipped
unit tests remain visible; new focused regressions and protocol-correct end-to-end
checks cover the changed lifecycle paths instead of claiming those skips passed.

## Reproduce

```sh
go test ./game -run '^$' -bench '^BenchmarkRoom' -benchmem -count 3
PONGO_CLIENTS=4 go test ./test -run '^TestPerformanceRooms$' -count=1 -v
PONGO_CLIENTS=40 go test ./test -run '^TestPerformanceRooms$' -count=1 -v
PONGO_CLIENTS=200 go test ./test -run '^TestPerformanceRooms$' -count=1 -v
PONGO_CLIENTS=40 PONGO_IDLE=1 go test ./test -run '^TestPerformanceRooms$' -count=1 -v
PONGO_CLIENTS=200 go test ./test -run '^TestPerformanceRooms$' -count=1 \
  -cpuprofile /tmp/pongo-cpu.prof -memprofile /tmp/pongo-memory.prof \
  -o /tmp/pongo-test-binary
go tool pprof -top /tmp/pongo-cpu.prof
go test -race ./... -count=1 -timeout 120s
```

Run performance commands sequentially on an otherwise idle host. This check does
not measure Internet latency, deployment proxies, browser rendering, multi-host
room routing, long-duration endurance, or the maximum sustainable production
player count. Per-client queues disconnect overload rather than silently dropping
events; disconnected clients cannot be promised a final message. Internal actor
queues are reliable in-process queues, not durable storage.
