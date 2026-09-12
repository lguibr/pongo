# Utilities package

Shared configuration and helpers.

- `config.go`: `Config`, the tunable game parameters, with `DefaultConfig()` and the test
  presets `E2ETestConfig()` and `BrickCollisionTestConfig()`.
- `constants.go`: `MaxPlayers` and the grid cell types (`Cells.Brick`, `Cells.Block`,
  `Cells.Empty`).
- `utils.go`: integer and vector maths, random vectors and colours, `DirectionFromString`
  (client arrow keys to paddle directions) and the `AssertPanics` test helper.

See [the game package](../game/README.md) and [the server package](../server/README.md).
