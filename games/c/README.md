# Snake &middot; C

C compiled straight to WebAssembly by clang: `--target=wasm32 -nostdlib`. No emscripten, no libc, no
runtime — wasm-ld links a module with no imports, and `__attribute__((export_name(...)))` decides
what comes out. The board is a static array that JavaScript reads directly from the module's linear
memory.

It shares its ABI with the [Rust version](../rust), so the two pages run the same renderer.

## Build

```bash
bash build.sh dist
```

Needs clang and lld with wasm support (`apt install clang lld` on Debian/Ubuntu). Serve `dist` over
HTTP — `WebAssembly.instantiate` needs a real response.

## Layout

| File | Purpose |
|---|---|
| `snake.c` | Game rules and the exported wasm interface |
| `index.html` | Canvas renderer, input handling and the frame loop |
| `build.sh` | Compiles and stages both into an output directory |

## The interface

`init(seed)`, `set_dir(0..3)` and `step()` drive the game; `score()`, `length()`, `alive()` and
`won()` report on it. `grid_ptr()` returns a pointer to `width() * height()` bytes, one per cell:
`0` empty, `1` body, `2` head, `3` food.
