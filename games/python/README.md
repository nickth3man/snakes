# Snake &middot; Python

Real CPython, compiled to WebAssembly by [Pyodide](https://pyodide.org). Nothing here is transpiled
to JavaScript: `main.py` runs on the interpreter — dataclasses, `random.choice` and all — and
reaches out to the DOM through the `js` bridge to draw on a canvas.

The cost is start-up. The interpreter is a few megabytes of download the first time the page is
opened, which is why the board shows a loading message before the first frame.

## Build

```bash
bash build.sh dist
```

There is nothing to compile — the script syntax-checks `main.py` and copies two files. Pyodide
itself is loaded from the jsDelivr CDN at runtime, so this page (alone among the six) needs network
access beyond the site itself.

## Layout

| File | Purpose |
|---|---|
| `main.py` | Everything: rules, input, frame loop and canvas drawing |
| `index.html` | Page chrome, the Pyodide loader and the call into `main.py` |
| `build.sh` | Syntax-checks and stages both |

`Game` is pure logic with no DOM in sight; `UI` is the part that talks to the browser. Callbacks
handed to JavaScript are wrapped in `create_proxy` and kept alive on the `UI` instance — a proxy
that gets garbage collected takes its event listener with it.
