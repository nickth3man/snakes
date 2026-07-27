# Snake &middot; Rust

Rust compiled to `wasm32-unknown-unknown` with no `std`, no allocator and no `wasm-bindgen`. The
whole board lives in a couple of statics inside linear memory; the module exports about ten
functions and imports nothing at all. JavaScript drives the clock, reads the grid straight out of
`memory` and paints it onto a canvas.

Rules, the three-tier AI, breadth-first search and flood fill all fit in a 29 KB `.wasm` file.

## What is in it

The full game, the same as every other language here: a menu with **normal** and **demo** modes,
the three-tier demo AI with its vision overlay, the direction queue, the HUD and high score, touch
controls and the juice. See the [repository README](../../README.md) for the rules and the AI.

## Build

```bash
bash build.sh dist
```

Needs rustup; the script adds the `wasm32-unknown-unknown` target itself. Serve `dist` over HTTP —
`WebAssembly.instantiate` needs a real response.

## Tests

```bash
cd ../typescript && npx tsx ai-parity.mts ../rust/target/wasm32-unknown-unknown/release/snake_rs.wasm
```

That plays full games in this module and compares every demo-AI decision — tier, move, reachable
region and planned path — against `getAIDirection` in the TypeScript source.

## Layout

| File | Purpose |
|---|---|
| `src/lib.rs` | Rules, demo AI and the exported wasm interface |
| `index.html` | Menu, HUD, canvas renderer, AI-vision overlay and input |
| `build.sh` | Compiles and stages both into an output directory |

## The interface

`init(seed)`, `set_dir(0..3)` and `step()` drive the game; `score()`, `length()`, `alive()` and
`won()` report on it. `grid_ptr()` returns a pointer to `width() * height()` bytes, one per cell:
`0` empty, `1` body, `2` head, `3` food.
