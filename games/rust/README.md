# Snake &middot; Rust

Rust compiled to `wasm32-unknown-unknown` with no `std`, no allocator and no `wasm-bindgen`. The
whole board lives in a couple of statics inside linear memory; the module exports about ten
functions and imports nothing at all. JavaScript drives the clock, reads the grid straight out of
`memory` and paints it onto a canvas.

The result is a ~3 KB `.wasm` file.

## Build

```bash
bash build.sh dist
```

Needs rustup; the script adds the `wasm32-unknown-unknown` target itself. Serve `dist` over HTTP —
`WebAssembly.instantiate` needs a real response.

## Layout

| File | Purpose |
|---|---|
| `src/lib.rs` | Game rules and the exported wasm interface |
| `index.html` | Canvas renderer, input handling and the frame loop |
| `build.sh` | Compiles and stages both into an output directory |

## The interface

`init(seed)`, `set_dir(0..3)` and `step()` drive the game; `score()`, `length()`, `alive()` and
`won()` report on it. `grid_ptr()` returns a pointer to `width() * height()` bytes, one per cell:
`0` empty, `1` body, `2` head, `3` food.
