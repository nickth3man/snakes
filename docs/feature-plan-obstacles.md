# Feature Plan: Static Obstacles (Procedural Walls)

## Brainstormed candidates (5)

1. **Static obstacles** — random scattered wall blocks at the start of each round
2. **Maze** — fixed pre-designed maze layout from a small hand-drawn set
3. **Moving hazards** — a few walls that slide around during play
4. **Destructible obstacles** — break walls on impact (with a body cost)
5. **Bombs** — timed explosions that clear a small radius

## Choice: Static obstacles

Why this one next:
- Builds on cycle 1 (walls live in `Game` alongside `wrap` and `occupied`)
- Pure rules change, no assets, no audio, no time-based logic
- Forces the AI's BFS and flood fill to dodge the new blocked cells
- Deterministic seeding keeps the game reproducible
- Trivially testable in `go test`; trivially renderable in the existing canvas
- `Edb83/snake` and `DarkSnakeGang/GoogleSnakeModLoader` both treat obstacles as a core feature

## Design

- A new `obstacles []Point` field on `Game`. The cell is "obstacle" iff
  `obstacles[i] != ""`. Stored as a `[]bool` of length `cols*rows` to
  match the existing `occupied` pattern.
- `NewGame(cols, rows, seed)` is extended with an `obstacles` count;
  the rules-only API keeps the old constructor by adding a new
  `NewGameWithObstacles(cols, rows, seed, n int)` constructor. A seed
  drives placement.
- Placement rules: never on the snake's starting cells, never on the
  food, never on the edges (so the open field stays walkable), and
  never adjacent to the head's first step (so the player is not born
  dead).
- `Step` checks obstacles before checking self-collision; an obstacle
  step is a death, exactly like a wall in non-wrap mode.
- `bfs` and `flood` mark obstacles as blocked.
- `Decide` is unaffected — it already works on the `blocked` slice.
- `wallAdjacent` in wrap mode now counts an obstacle as a wall too.

## UI

- `index.html`: new `#play-obstacles` card (amber), new `#obstacles-pill`
  HUD pill (amber border), `O` key in menu and in-game.
- `main.go`: `toggleObstacles`, `obstacles bool`, `obstaclesBtn`,
  `obstaclesPill js.Value`. Per-round `NewGameWithObstacles` when on.
- The HUD pill shows "WALLS" or "WALLS + OBSTACLES" or "WALLS" depending
  on whether wrap is on; obstacles get their own amber pill.

## Tests

- `TestObstaclePlacementAvoidsSnakeAndEdges`
- `TestObstaclePlacementAvoidsHeadNeighborhood`
- `TestObstaclePlacementHonoursCount`
- `TestObstacleStepIsFatal`
- `TestObstacleBFSBlocked` — BFS returns nil when only an obstacle is in the way
- `TestObstacleFloodIsSmaller` — flood fill count is reduced by obstacles
- `TestObstacleDecideAvoids` — AI never picks a move that hits an obstacle
- `TestObstaclesDeterministic` — same seed → same obstacle positions
- `TestDecideMatchesReference` — must remain green (no regression)
- `TestGameRules` — must remain green
- All cycle-1 wrap tests must remain green (independent feature)

## Verification

- `go test ./...` — all green
- `GOOS=js GOARCH=wasm go build` — clean
- `bash build.sh dist` — clean
- Chrome DevTools MCP — fresh session, document.body must not be empty.
  If still wedged, fall back to asserting HTML+wasm+exec via curl and
  call out the manual interactivity step in the commit.
