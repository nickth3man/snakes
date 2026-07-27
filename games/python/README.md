# Snake &middot; Python

Real CPython, compiled to WebAssembly by [Pyodide](https://pyodide.org). Nothing here is transpiled
to JavaScript: `main.py` runs on the interpreter — dataclasses, `random.choice` and all — and
reaches out to the DOM through the `js` bridge to draw on a canvas.

The cost is start-up. The interpreter is a few megabytes of download the first time the page is
opened, which is why the board shows a loading message before the first frame.

## What is in it

The full game, the same as every other language here: a menu with **normal** and **demo** modes,
the three-tier demo AI with its vision overlay, the direction queue, the HUD and high score, touch
controls and the juice. See the [repository README](../../README.md) for the rules and the AI.

## Build

```bash
bash build.sh dist
```

There is nothing to compile — the script syntax-checks `main.py` and copies two files. Pyodide
itself is loaded from the jsDelivr CDN at runtime, so this page (alone among the six) needs network
access beyond the site itself.

## Tests

```bash
python -m unittest discover -s games/python    # from the repository root
```

`test_ai.py` stubs out the browser modules, imports `main.py`, replays the board states in
[`testdata/ai-trace.json`](../../testdata/ai-trace.json) and demands the same answers the original
produced, plus a few rule checks the trace cannot cover.

## Layout

| File | Purpose |
|---|---|
| `main.py` | Everything: rules, demo AI, input, frame loop and canvas drawing |
| `test_ai.py` | Trace and rule tests, run on the host interpreter |
| `index.html` | Page shell, the Pyodide loader and the call into `main.py` |
| `build.sh` | Syntax-checks and stages the page |

`Game` is pure logic with no DOM in sight; `UI` is the part that talks to the browser. Callbacks
handed to JavaScript are wrapped in `create_proxy` and kept alive on the `UI` instance — a proxy
that gets garbage collected takes its event listener with it.
