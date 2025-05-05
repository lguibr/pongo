# PonGo E2E Tests

This directory contains End-to-End (E2E) tests for the PonGo backend. These tests simulate client interactions with the running server by connecting via WebSocket and verifying the sequence and content of messages received from the server.

## Running Tests

Run all tests in this directory:

```bash
go test ./test -v
```

Run only E2E tests (useful if unit tests are separate):

```bash
go test ./test -v -run E2E
```

Run with the race detector (recommended):

```bash
go test ./test -v -race
```

## Tests

*   **`e2e_test.go`**:
    *   `TestE2E_SinglePlayerConnectMoveStopDisconnect`: Simulates a single player connecting, receiving initial state, sending move and stop commands, and disconnecting. Verifies the sequence of messages and basic state updates (e.g., paddle `IsMoving` flag) received via atomic `GameUpdatesBatch` messages.
    *   `TestE2E_BallWallNonStick`: Verifies that balls correctly move away from walls after a collision. It monitors `BallPositionUpdate` messages, detects when a ball's `Collided` flag is true near a wall, and then checks subsequent updates to ensure the ball's position changes away from that wall.
*   **`stress_test.go`**:
    *   `TestE2E_StressTestMultipleRooms`: Simulates multiple concurrent clients connecting and sending random movement commands for a sustained period. This test checks server stability and performance under load, including handling concurrent connections, actor message passing, and actor shutdowns. It primarily asserts that a high percentage of clients connect successfully and the server doesn't crash.
*   **`stress_game_completion_test.go`**:
    *   `TestE2E_StressTestGameCompletion`: Simulates many clients connecting and waiting for their respective games to finish (all bricks destroyed). Uses an ultra-fast game configuration (`utils.UltraFastGameConfig`) to accelerate completion. Checks how many games successfully finish (clients receive `GameOverMessage`) within the test duration, asserting a minimum completion rate.

## Profiling E2E Tests

You can run the E2E tests with Go's built-in profiling enabled to identify performance bottlenecks during test execution.

1.  **Run with Profiling Flags:**
    The `build_test.sh` script includes a step for this, or you can run it manually:
    ```bash
    # Example: Profile the stress test
    go test ./test -v -race -run "^TestE2E_StressTestMultipleRooms$" -timeout 90s -cpuprofile cpu.prof -memprofile mem.prof
    ```
    This will execute the specified test(s) and generate `cpu.prof` (CPU profile) and `mem.prof` (memory allocation profile) files in the project root directory.

2.  **Analyze Profiles:**
    Use the `go tool pprof` command to analyze the generated profiles.
    *   **CPU Profile:**
        ```bash
        go tool pprof cpu.prof
        ```
        Inside the pprof tool, common commands include `top` (show functions consuming the most CPU), `web` (generate a graphical representation - requires graphviz), `list <function_name>` (show source code annotation for a function).
    *   **Memory Profile:**
        ```bash
        # Analyze allocations in use at the end of the test
        go tool pprof mem.prof
        # Analyze total allocations throughout the test run
        go tool pprof -alloc_objects mem.prof
        ```
        Similar commands (`top`, `web`, `list`) apply. Use `top -cum` to see cumulative allocations.

## Related Modules

*   [Game Logic](../game/README.md)
*   [Server](../server/README.md)
*   [Main Project](../README.md)
