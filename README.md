# snakes

The same Snake game, written six times in six programming languages, all playable in the browser.

**[Play them &rarr;](https://nickth3man.github.io/snakes/)**

Every version follows the same rules — a 24&times;24 board, walls are fatal, the tail is not, ten
points an apple, and the clock speeds up as you grow — and uses the same palette. What changes is
how the board gets from source code to your screen.

| Language | Approach | Source |
|---|---|---|
| [TypeScript](https://nickth3man.github.io/snakes/typescript/) | Phaser 3 scenes over a tested engine module, bundled by Vite | [`games/typescript`](games/typescript) |
| [Rust](https://nickth3man.github.io/snakes/rust/) | `no_std`, no wasm-bindgen; a ~3&nbsp;KB module keeps the board in linear memory and JavaScript paints it | [`games/rust`](games/rust) |
| [Go](https://nickth3man.github.io/snakes/go/) | `GOOS=js GOARCH=wasm`; Go drives input, the frame loop and the canvas through `syscall/js` | [`games/go`](games/go) |
| [C](https://nickth3man.github.io/snakes/c/) | Freestanding `clang --target=wasm32 -nostdlib` — no libc, no emscripten, no runtime | [`games/c`](games/c) |
| [Python](https://nickth3man.github.io/snakes/python/) | Real CPython compiled to WebAssembly by Pyodide, calling the DOM through the `js` bridge | [`games/python`](games/python) |
| [Elm](https://nickth3man.github.io/snakes/elm/) | No canvas and no draw loop: the board is a value, the view is a pure function, the output is SVG | [`games/elm`](games/elm) |

## Building

Each game builds on its own and stages its output into a directory you name:

```bash
bash games/rust/build.sh /tmp/out/rust
```

To build the whole site the way CI does:

```bash
mkdir -p _site && cp index.html _site/ && touch _site/.nojekyll && for game in typescript rust go c python elm; do bash "games/$game/build.sh" "$PWD/_site/$game"; done
```

Then serve `_site` over HTTP (`python -m http.server --directory _site`) — the WebAssembly builds
need real HTTP responses, so opening the files directly will not work. The TypeScript build is the
one exception to "just serve it": its production bundle is built for the `/snakes/typescript/` path,
so use `npm run dev` inside [`games/typescript`](games/typescript) when working on it locally.

### What each build needs

| Game | Toolchain |
|---|---|
| typescript | Node 20+ |
| rust | rustup with the `wasm32-unknown-unknown` target |
| go | Go 1.24+ |
| c | clang and lld with wasm support |
| python | Python 3 (only to syntax-check; Pyodide is loaded from a CDN at runtime) |
| elm | Node 20+ (the Elm compiler is fetched through `npx`) |

## Deployment

[`.github/workflows/pages.yml`](.github/workflows/pages.yml) builds all six games plus the hub page
into a single artifact and publishes it to GitHub Pages on every push to `main`. Pull requests run
the same build without deploying. The Pages source is set to GitHub Actions.

## Adding a seventh

1. Create `games/<language>/` with an `index.html` and your source.
2. Add a `build.sh` that takes an output directory and stages a ready-to-serve page into it.
3. Add a build step to [`.github/workflows/pages.yml`](.github/workflows/pages.yml) and the language
   to the list it verifies.
4. Add a card to [`index.html`](index.html).

## License

MIT — see [LICENSE](LICENSE).
