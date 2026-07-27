# Snake &middot; Elm

The odd one out. There is no canvas and no imperative draw loop: the board is a value, `update`
returns the next board, and the view is a pure function from that value to SVG. Elm's runtime
handles the rest.

Side effects are limited to what Elm makes explicit — a `Time.every` subscription for the tick, a
`Browser.Events.onKeyDown` subscription for input, and one port that hands the high score back to
JavaScript for `localStorage`. Randomness is a `Random.Seed` threaded through the model, so a game
is reproducible from its seed.

## What is in it

The full game, the same as every other language here: a menu with **normal** and **demo** modes,
the three-tier demo AI with its vision overlay, the direction queue, the HUD and high score, touch
controls and the juice. See the [repository README](../../README.md) for the rules and the AI.

## Build

```bash
bash build.sh dist
```

Needs Node 20+; the Elm compiler is fetched through `npx` so there is no toolchain to install.

## Tests

```bash
node tests/verify.mjs
```

`tests/Verify.elm` is a `Platform.worker` that replays the board states in
[`testdata/ai-trace.json`](../../testdata/ai-trace.json) through `Ai.decide` and reports any
disagreement with the original. The runner compiles it and feeds it the trace under Node.

## Layout

| File | Purpose |
|---|---|
| `src/Board.elm` | Positions, directions and the board both halves reason about |
| `src/Ai.elm` | The three-tier demo AI, with BFS and flood fill |
| `src/Main.elm` | Model, update, subscriptions and the SVG view of both screens |
| `tests/Verify.elm` | Headless worker that checks the AI against the trace |
| `elm.json` | Pinned dependencies |
| `index.html` | Page shell, flags in (high score, seed, viewport) and the port out |
| `build.sh` | Compiles with `--optimize` and stages the page |

Elm renders both screens itself — menu, HUD, board and overlays — so the HTML file is only a
shell with the stylesheet in it.
