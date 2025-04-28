
# Utilities Module

This module provides shared constants, configuration management, helper functions, and basic types used across the PonGo backend modules.

## Key Components

*   **`config.go`**:
    *   Defines the `Config` struct holding all tunable game parameters (timing, physics, sizes, power-ups, etc.).
    *   Provides `DefaultConfig()` to get a standard configuration set.
    *   *(Future: Could include functions to load config from files).*
*   **`constants.go`**:
    *   Defines fundamental constants like `MaxPlayers`.
    *   Defines `CellType` enum (`Brick`, `Block`, `Empty`) and `Cells` helper struct.
    *   Contains **deprecated** constants that are now sourced from `Config`.
*   **`utils.go`**:
    *   Mathematical helpers (`MaxInt`, `MinInt`, `Abs`).
    *   Vector/Matrix operations (mostly for grid generation, potentially physics).
    *   Random number/color generation (`NewRandomColor`, `RandomNumber`, etc.).
    *   String conversion (`DirectionFromString`).
    *   Testing helpers (`AssertPanics`).
    *   Logging helpers (`JsonLogger`, `Logger`).

## Related Modules

*   [Game Logic](../game/README.md)
*   [Server](../server/README.md)
*   [Main Project](../README.md)