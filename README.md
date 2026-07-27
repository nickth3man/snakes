# snakes

The same Snake game, written six times in six programming languages, all playable in the browser.

**[Play them &rarr;](https://nickth3man.github.io/snakes/)**

Every version is the same game, feature for feature: the same menu with **normal** and **demo**
modes, the same 30&times;22 board (18&times;38 in portrait), the same rules (walls and your own body
are fatal, one point an apple, one move every 130 ms), the same HUD, high score, touch controls and
the same juice — and the same three-tier demo AI, with a vision overlay that draws its flood fill
and planned path onto the board. What changes is how all that gets from source code to your screen.

| Language | Approach | Source |
|---|---|---|
| [TypeScript](https://nickth3man.github.io/snakes/typescript/) | Phaser 3 scenes over a tested engine module, bundled by Vite — the original the other five are ported from | [`games/typescript`](games/typescript) |
| [Rust](https://nickth3man.github.io/snakes/rust/) | `no_std`, no wasm-bindgen; rules, AI and search in 29&nbsp;KB of wasm, with JavaScript painting what it finds in linear memory | [`games/rust`](games/rust) |
| [Go](https://nickth3man.github.io/snakes/go/) | `GOOS=js GOARCH=wasm`; Go drives input, the frame loop and the canvas through `syscall/js` | [`games/go`](games/go) |
| [C](https://nickth3man.github.io/snakes/c/) | Freestanding `clang --target=wasm32 -nostdlib` — no libc, no emscripten, no runtime | [`games/c`](games/c) |
| [Python](https://nickth3man.github.io/snakes/python/) | Real CPython compiled to WebAssembly by Pyodide, calling the DOM through the `js` bridge | [`games/python`](games/python) |
| [Elm](https://nickth3man.github.io/snakes/elm/) | No canvas and no draw loop: the board is a value, the view is a pure function, the output is SVG | [`games/elm`](games/elm) |

## Controls

| | |
|---|---|
| <kbd>N</kbd> / <kbd>D</kbd> | start a normal or demo game from the menu |
| <kbd>←↑↓→</kbd> / <kbd>WASD</kbd> | steer (two moves may be buffered) |
| <kbd>V</kbd> | in demo mode, show what the AI is thinking |
| <kbd>M</kbd> | back to the menu |
| <kbd>Space</kbd> / tap | restart after a game over |

On a touch device, swipe to steer and use the D-pad in landscape.

## The demo AI

All six share one algorithm, ported from
[`demo-controller.ts`](games/typescript/src/ai/demo-controller.ts):

1. **Tier 1 — safe pursuit.** Breadth-first search to the food, but only from moves that leave at
   least 1.2&times; the snake's length of reachable cells. Ties prefer the shorter path, then more
   wall-hugging, then more turns, because it looks better.
2. **Tier 2 — max space.** Flood fill each option and take the roomiest. Ties prefer wall-adjacent
   cells, then whatever lands nearer the food.
3. **Fallback.** Lunge at the food and hope.

Turn on AI Vision to see the flood-filled region shaded and the planned path dashed, with a badge
naming the tier in play.

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

## Tests

Six implementations of one algorithm is six chances to drift, so every port is held to the same
recorded answers. [`testdata/ai-trace.json`](testdata/ai-trace.json) holds 259 board states with the
tier, move, reachable-region size and path length the original produced for each — covering all
three tiers — and each port must reproduce them exactly.

```bash
cd games/typescript && npm test                                            # engine + layout units
cd games/typescript && npx tsx ai-parity.mts ../rust/target/wasm32-unknown-unknown/release/snake_rs.wasm
cd games/go && go test ./...                                               # trace + rules
python -m unittest discover -s games/python                                # trace + rules
cd games/elm && node tests/verify.mjs                                      # trace
```

`ai-parity.mts` is the strictest of these: it plays full games in the wasm module and compares every
single decision against `getAIDirection` from the TypeScript source, rather than a recording. The
same script checks the C build, since C and Rust export the same interface. CI runs all of it.

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
