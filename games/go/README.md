# Snake &middot; Go

Go built with `GOOS=js GOARCH=wasm`. Unlike the [Rust](../rust) and [C](../c) versions — which
export a grid and let JavaScript paint it — this one owns the whole frame: `syscall/js` reads the
keyboard, drives `requestAnimationFrame` and draws to the canvas. The JavaScript on the page does
nothing but boot the Go runtime.

That runtime is the trade-off: the binary is around 2.7 MB (roughly 600 KB over the wire).

## Build

```bash
bash build.sh dist
```

Needs Go 1.24+. The script also copies `wasm_exec.js` out of `GOROOT`, since that shim ships with
the toolchain and has to match the compiler that built the binary. Serve `dist` over HTTP.

## Layout

| File | Purpose |
|---|---|
| `main.go` | Everything: rules, input, frame loop and canvas drawing |
| `index.html` | Page chrome and the loader that starts the Go runtime |
| `build.sh` | Compiles, copies `wasm_exec.js` and stages the page |

The `game` type is pure logic with no DOM in sight; `ui` is the part that talks to the browser.
