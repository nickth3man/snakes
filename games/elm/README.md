# Snake &middot; Elm

The odd one out. There is no canvas and no imperative draw loop: the board is a value, `update`
returns the next board, and the view is a pure function from that value to SVG. Elm's runtime
handles the rest.

Side effects are limited to what Elm makes explicit — a `Time.every` subscription for the tick, a
`Browser.Events.onKeyDown` subscription for input, and one port that hands the high score back to
JavaScript for `localStorage`. Randomness is a `Random.Seed` threaded through the model, so a game
is reproducible from its seed.

## Build

```bash
bash build.sh dist
```

Needs Node 20+; the Elm compiler is fetched through `npx` so there is no toolchain to install.

## Layout

| File | Purpose |
|---|---|
| `src/Main.elm` | The entire game: model, update, subscriptions and SVG view |
| `elm.json` | Pinned dependencies |
| `index.html` | Page chrome, flags in (high score, seed) and the port out |
| `build.sh` | Compiles with `--optimize` and stages the page |

Note that Elm renders the header, board and overlay itself — the HTML file is only a shell.
