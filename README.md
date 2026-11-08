# Go Terminal Game of Life

A terminal implementation of Conway's Game of Life (and Life-like cellular automata) with built-in SQLite telemetry, snapshot persistence, and state-cycle detection, built with the Go standard library and pure-Go SQLite (`modernc.org/sqlite`).

## Architecture & Design

- **Contiguous 1D Memory:** Represents the 2D board as a flat `[]uint8` slice in row-major order (`y*Width + x`) for CPU cache locality.
- **Double Buffering:** Swaps pre-allocated `Current` and `Next` slices each tick to keep the simulation loop allocation-free.
- **Bitwise Rule Engine:** Evaluates state transitions via a hexadecimal bitmask (`0x1808` for Conway's **B3/S23**) instead of branching `if/else` chains.
- **Buffered ANSI Rendering:** Assembles each frame in memory and redraws in place using ANSI cursor/line control sequences (`\033[1A`, `\033[2K`).
- **Relational Telemetry & Snapshots:** Logs run metadata, FNV-1a state hashes, and batched `WITHOUT ROWID` tick series in SQLite, supporting `BLOB` board saves and `LAG()` window analytics.

## Quick Start & CLI Usage

```bash
# Run default simulation (with optional cycle detection)
go run .
go run . -cycle

# Save a run's final state or load an existing snapshot
go run . save -name glider
go run . load -name glider -sa    # -sa (-save-after) updates the snapshot on exit

# Inspect saved snapshots, view SQLite telemetry stats, or delete a save
go run . list
go run . stats
go run . delete -name glider

# Run SQLite write-path benchmarks (Default vs. WAL vs. WAL + NORMAL sync)
go test -bench=. -benchmem ./bench
```

## Rule Bitmask Format

Rules are encoded in an 18-bit integer where bit positions correspond to neighbor counts (`0–8`):
- **Bits 0–8:** Birth conditions (dead cell with $N$ live neighbors).
- **Bits 9–17:** Survival conditions (live cell with $N$ live neighbors, offset by 9).

| Automaton | Rule Notation | Hex Mask |
| :--- | :--- | :--- |
| **Conway's Life** | B3/S23 | `0x1808` |
| **HighLife** | B36/S23 | `0x1848` |
| **Seeds** | B2/S | `0x0004` |
| **Day & Night** | B3678/S34678 | `0x3B1C8` |

## Roadmap

- [x] Graceful `Ctrl+C` shutdown via `os/signal` to restore terminal state
- [x] SQLite schema and batched telemetry persistence (`configs`, `runs`, `ticks`, `saves`)
- [x] Subcommand CLI router (`save`, `load`, `list`, `stats`, `delete`) and `-cycle` detection
- [x] Package modularization (`model`, `store`, `bench`) and WAL/synchronous benchmark suite
- [ ] Concurrent Headless Hunter: Multi-goroutine worker pool to hunt for long-lived Methuselah seeds and interesting 18-bit rules, saving record-breaking Frame 0 snapshots to SQLite
- [ ] Video & Image Pipeline: Export headless runs as binary `.ppm` frame sequences and invoke `ffmpeg` via `os/exec` (or encode animated GIFs via `image/gif`) to visualize large grids beyond terminal limits
- [ ] Langton's Ant: Autonomous 2D agent with composite state-vector cycle detection
- [ ] CLI flags (`flag` package) for grid dimensions, tick speed, and rule masks
