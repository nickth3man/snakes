# Snake &middot; C

C compiled straight to WebAssembly by clang: `--target=wasm32 -nostdlib`. No emscripten, no libc, no
runtime — wasm-ld links a module with no imports, and `__attribute__((export_name(...)))` decides
what comes out. The board is a static array that JavaScript reads directly from the module's linear
memory.

It shares its ABI with the [Rust version](../rust), so the two pages run the same renderer.

## What is in it

The full game, the same as every other language here: a menu with **normal** and **demo** modes,
the three-tier demo AI with its vision overlay, the direction queue, the HUD and high score, touch
controls and the juice. See the [repository README](../../README.md) for the rules and the AI.

## Build

```bash
bash build.sh dist
```

Needs clang and lld with wasm support (`apt install clang lld` on Debian/Ubuntu). Serve `dist` over
HTTP — `WebAssembly.instantiate` needs a real response.

## Tests

```bash
cd ../typescript && npx tsx ai-parity.mts ../../_site/c/snake.wasm
```

Because this module exports the same interface as the Rust one, the same harness checks it: it
plays full games and compares every demo-AI decision against the TypeScript source.

## Layout

| File | Purpose |
|---|---|
| `snake.c` | Rules, demo AI and the exported wasm interface |
| `index.html` | Menu, HUD, canvas renderer, AI-vision overlay and input |
| `build.sh` | Compiles and stages both into an output directory |

## The interface

`init(seed)`, `set_dir(0..3)` and `step()` drive the game; `score()`, `length()`, `alive()` and
`won()` report on it. `grid_ptr()` returns a pointer to `width() * height()` bytes, one per cell:
`0` empty, `1` body, `2` head, `3` food.
