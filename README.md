# Go Terminal Game of Life

A zero-dependency terminal implementation of Conway's Game of Life (and Life-like cellular automata) built using only the Go standard library.

## Architecture & Design

* **Contiguous 1D Memory:** Represents the 2D board as a flat `[]uint8` slice in row-major order (`y*Width + x`) for CPU cache locality.
* **Double Buffering:** Swaps pre-allocated `Current` and `Next` slices each tick to keep the simulation loop allocation-free.
* **Bitwise Rule Engine:** Evaluates state transitions via a hexadecimal bitmask (`0x1808` for Conway's **B3/S23**) instead of branching `if/else` chains.
* **Buffered ANSI Rendering:** Assembles each frame in memory and redraws in place using ANSI cursor/line control sequences (`\033[1A`, `\033[2K`).

## Quick Start

```bash
# Run directly
go run .

# Build and run binary
go build -o gol .
./gol
```

## Rule Bitmask Format

Rules are encoded in an 18-bit integer where bit positions correspond to neighbor counts (`0–8`):
* **Bits 0–8:** Birth conditions (dead cell with $N$ live neighbors).
* **Bits 9–17:** Survival conditions (live cell with $N$ live neighbors, offset by 9).

| Automaton | Rule Notation | Hex Mask |
| :--- | :--- | :--- |
| **Conway's Life** | B3/S23 | `0x1808` |
| **HighLife** | B36/S23 | `0x1848` |
| **Seeds** | B2/S | `0x0004` |
| **Day & Night** | B3678/S34678 | `0x3B1C8` |

## Roadmap

- [ ] Graceful `Ctrl+C` shutdown via `os/signal` to restore terminal state
- [ ] CLI flags (`flag` package) for grid dimensions, tick speed, and rule masks