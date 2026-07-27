# Snake &middot; Go

Go built with `GOOS=js GOARCH=wasm`. Unlike the [Rust](../rust) and [C](../c) versions — which
export a grid and let JavaScript paint it — this one owns the whole frame: `syscall/js` reads the
keyboard, drives `requestAnimationFrame` and draws to the canvas. The JavaScript on the page does
nothing but boot the Go runtime.

That runtime is the trade-off: the binary is around 2.7 MB (roughly 600 KB over the wire).

## What is in it

The full game, the same as every other language here: a menu with **normal** and **demo** modes,
the three-tier demo AI with its vision overlay, the direction queue, the HUD and high score, touch
controls and the juice. See the [repository README](../../README.md) for the rules and the AI.

## Build

```bash
bash build.sh dist
```

Needs Go 1.24+. The script also copies `wasm_exec.js` out of `GOROOT`, since that shim ships with
the toolchain and has to match the compiler that built the binary. Serve `dist` over HTTP.

## Tests

```bash
go test ./...
```

`ai_test.go` replays the board states in [`testdata/ai-trace.json`](../../testdata/ai-trace.json)
and demands the same tier, move, reachable region and path length the original produced, plus a few
rule checks the trace cannot cover. `main.go` is behind a `js && wasm` build tag, so the rules and
the AI compile and test natively.

## Layout

| File | Purpose |
|---|---|
| `game.go` | The rules: board, snake, food, direction queue |
| `ai.go` | The three-tier demo AI, with BFS and flood fill |
| `main.go` | Menus, input, frame loop, HUD and canvas drawing (wasm only) |
| `ai_test.go` | Trace and rule tests, run natively |
| `index.html` | Page shell and the loader that starts the Go runtime |
| `build.sh` | Compiles, copies `wasm_exec.js` and stages the page |

`game.go` and `ai.go` are pure logic with no DOM in sight, which is what lets `go test` run them
on the host; `main.go` is the part that talks to the browser.
