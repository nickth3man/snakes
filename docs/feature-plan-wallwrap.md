# Feature Plan: Wall-Wrap (Torus) Mode

## Brainstormed candidates (5)

1. **Wall-wrap (torus) mode** — snake passes through walls to the opposite side
2. **Power-ups** — speed boost, slow-mo, ghost, score multiplier spawn randomly
3. **Static obstacles / maze** — interior walls that the AI must navigate around
4. **Sound effects** — Web Audio API eat / die / win chimes
5. **Time-attack mode** — countdown clock; each food adds time

## Choice: Wall-wrap (torus) mode

Why this one first:
- Self-contained rule change: no new entities, no assets
- Forces the AI to be wrap-aware (BFS + flood fill must traverse edges)
- Has a clear UI toggle and a short keyboard shortcut (`W`)
- Trivially testable in `go test` (movement, BFS, flood)
- End-to-end verifiable in the browser through Chrome DevTools

## Implementation plan

### `game.go` — pure rules
- Add `wrap bool` field to `Game`
- Add `SetWrap(bool)` method
- Add helper `wrapPoint(p Point) Point` that normalises `X` to `[0, cols)` and `Y` to `[0, rows)`
- `Step()`: when `wrap` is on, run the next position through `wrapPoint` instead of treating out-of-bounds as death
- `inBounds` is unchanged (wrapping still has a logical board); `wallAdjacent` treats a cell that has no in-bounds neighbour as wall-adjacent only in wrap mode (so a snake never dies into a wall, the edge just becomes a real wall-equivalent)
- BFS / flood become wrap-aware via a `wrap` flag on the cell expansion

### `ai.go` — three-tier demo AI
- BFS / flood / `Decide` route through a `wrap bool` so neighbours can wrap
- `wallHugs` unchanged in spirit, but counts an edge cell whose opposite edge is reachable as a wall-hug
- Tier-1, Tier-2, Fallback all benefit automatically once BFS + flood are wrap-aware
- Trace test (`TestDecideMatchesReference`) stays green because it runs with `wrap=false`

### `main.go` — browser
- `ui` gains `wrap bool`, `wrapBtn js.Value`, `wrapHintEl js.Value`
- `start(m mode)` honours the toggle; new game keeps the same flag
- `bind()` wires the toggle, the `W` key, and a menu card
- HUD shows a small `wrap` pill when on

### `index.html`
- New menu card: `WRAP` (purple-bordered like DEMO)
- New HUD pill `#wrap-pill` shown only in wrap mode
- CSS for the new card mirrors the existing `.card` pattern

## Tests

- `TestWrapMovement` — head wraps right→left, top→bottom, with and without eating
- `TestWrapSurvivesBodyWrap` — long snake in tight board survives the wrap correctly (head lands where tail just left)
- `TestWrapBFSCrossesEdge` — BFS finds a path that requires leaving one edge and entering the opposite
- `TestWrapFloodCountsAllCells` — flood fill on an empty wrap board returns `cols*rows` reachable cells
- `TestDecideMatchesReference` — must remain green (no regression in non-wrap mode)
- `TestGameRules` — must remain green (no regression in non-wrap mode)

## Verification

- `go test ./...` — all green
- `bash build.sh dist` — produces `dist/snake.wasm`, `dist/wasm_exec.js`, `dist/index.html`
- Serve `dist/` over HTTP (e.g. `python -m http.server 8000 --directory dist`)
- Chrome DevTools MCP:
  1. `navigate_page` to `http://localhost:8000/`
  2. `take_snapshot` — confirm menu shows Normal / Demo / Wrap
  3. `click` Wrap → start wrap mode
  4. Drive the snake into the right wall; expect head to emerge on the left
  5. Toggle off; expect normal walls
