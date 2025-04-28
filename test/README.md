# PonGo E2E Tests

This directory contains End-to-End (E2E) tests for the PonGo backend. These tests simulate client interactions with the running server.

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
    *   `TestE2E_SinglePlayerConnectMoveStopDisconnect`: Simulates a single player connecting, sending move and stop commands, and disconnecting. Verifies basic state updates and connection handling.
*   **`stress_test.go`**:
    *   `TestE2E_StressTestMultipleRooms`: Simulates multiple concurrent clients connecting and sending random movement commands for a sustained period. This test is designed to check server stability and performance under load. It doesn't have strict gameplay assertions but checks for crashes, deadlocks, and basic connection success rates. Run this test without the `-short` flag. Monitor server logs during the test for potential issues.

## Related Modules

*   [Game Logic](../game/README.md)
*   [Server](../server/README.md)
*   [Main Project](../README.md)