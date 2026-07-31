# Feature Plan: Portal Mode (cycle 4)

## Choice

**Portal mode** — two paired teleporters placed deterministically on
the board. Stepping onto portal A teleports the head (and the rest of
the body, in a normal advance) to portal B. Matched-color `◉` glyphs
drawn at the cell centers, with a thin dotted line between them.

Inspired by the 9-mode `aayush301/Multi-mode-snake-game` and Google
Snake's Portal mode (per Reddit/TikTok walkthroughs). Both
implementations use linked pairs of fixed cells; ours matches the
"two paired glyphs" pattern.

## Scope decision: Go-first, not parity

The repo README requires "full TypeScript feature set parity for all
six ports" (TS, Rust, Go, C, Python, Elm). I grepped the entire
`games/typescript/src` tree: zero hits for `portal`, `teleport`, or
`◉`. The TypeScript engine has no portal mode. Cycle 4 ships a new
Go-first feature; the same rules and a shared placement fixture can
drive a future TS port. Calling this out so the parity invariant is
explicitly relaxed for cycle 4, not silently broken.

## Why portal mode

- **Self-contained grid mechanic.** No new rendering pipeline; the
  two glyphs reuse the existing cell-draw layer.
- **Deterministic placement.** `PickPortalPair(cols, rows, seed)` is
  pure and seed-driven, so the AI benchmark numbers and the
  Playwright/browser probes see the same layout.
- **AI-perturbing.** The BFS shortest-path now must consider
  "step on portal, teleport, step toward food" jumps. Real algorithmic
  change in `Decide` / `bfs` / `flood`.
- **Composable.** Portals compose cleanly with wrap (cycle 1) and
  obstacles (cycle 2). Future cycle can stress all 3 toggled on.
- **Test surface.** Pure placement + deterministic teleport is the
  cleanest parity fixture in the project so far.

## Rules (precise)

1. **Placement** (`PickPortalPair`, runs once per round):
   - Two distinct cells, neither on the initial snake (3 cells),
     nor on the food, nor on an obstacle (if obstacles enabled),
     nor on the head's 3×3 neighborhood.
   - Manhattan distance ≥ `minPairDist = 4` so the loop
     "A→B→A→B" cannot happen in a single tick.
   - Seed: `time.Now().UnixNano() ^ int64(cols)*int64(rows+7) ^
     int64(obsCount<<8) ^ 0xC0FFEE`.
   - If the board is too crowded to find a valid pair (no
     placement passes all filters), the round proceeds **without
     portals** — the feature is opt-in and never breaks the game.
2. **Trigger** (in `Step`):
   - When the head *enters* a portal cell, the candidate head
     becomes the partner cell. If the partner is an obstacle
     → die (wall). If the partner is a body cell (excluding the
     tail, which vacates this tick) → die (self). Otherwise
     accept: head lands on partner, body follows with the same
     delta as a normal advance (tail vacates, all segments shift).
   - A portal does NOT loop back through itself. The head can only
     enter each portal cell once per advance; a body segment
     sliding into a portal cell mid-advance (after the head
     teleported elsewhere) does NOT teleport.
3. **Wrap interaction.** If wrap is on, the post-teleport head is
   the literal partner cell, not a wrap-shifted version. Wrap
   continues to apply to the *post-teleport* position only — i.e.
   if the partner cell is at the wall, the next advance may wrap.
4. **Obstacle interaction.** Portal cells are never on obstacles
   (placement rule). Stepping into the partner as a normal move
   (not via teleport) is allowed — obstacles only block the
   *head's* advance, not body cells sliding through.
5. **Persistence.** No localStorage. The pair is recomputed at
   every `start()`. Mid-round moves do not move the portals.
6. **Win/loss.** Unchanged. Win at `cols*rows - obs` cells; lose
   on wall/self collision (teleport-into-wall-or-self counts as
   wall/self).

## Design (Go)

### `portals.go` — pure logic, host-testable, no `syscall/js`

- `type Portal struct { A, B Point }` (zero value = disabled)
- `func (p Portal) Enabled() bool { return p.A != p.B || p.A.X != 0 }`
  (canonical empty: `Point{-1,-1}, Point{-1,-1}`)
- `func (p Portal) At(pt Point) bool` — true if pt is A or B
- `func (p Portal) Partner(pt Point) Point` — returns the partner
  of pt, or pt itself if pt is not a portal cell
- `func PickPortalPair(cols, rows int, seed int64, blocked func(Point) bool) (Portal, bool)`
  — pure placement; returns `(zero, false)` if no valid pair exists
- `PortalMinPairDist = 4` — constant exported for the tests

### `game.go` — extend `Game` with portals

- Add `portals Portal` field
- Add `SetPortals(p Portal)`, `Portals() Portal`,
  `PortalAt(Point) (Point, bool)` (returns partner if portal,
  else zero+false)
- `NewGameWithPortals(cols, rows, seed int64, obsCount int, portals Portal) *Game`
  — factory; if `portals` zero-valued, behaves like
  `NewGameWithObstacles`
- In `Step`:
  - compute `next = head + dir` (with wrap if applicable)
  - if `portals.At(next)`, `next = portals.Partner(next)`
  - check `obstacleAt(next)` → die
  - check `occupied[next] && next != tail` → die
  - else: append head, if not eating → drop tail

### `ai.go` — `bfs` / `flood` get a portal-jump neighbor

- In the neighbor loop, after adding the 4 cardinals (and the
  wrap-shifted duplicates when wrap is on), if the current cell
  is portal A, also enqueue B; if portal B, also enqueue A. Same
  edge cost (1 tick) and the same obstacle / self-collision rules
  (the BFS only computes reachable cells, not a full path).
- `Decide`'s primary "go toward food" decision will then naturally
  pick the portal jump when the partner is closer to the food.

### `main.go` — wire UI

- `portals bool` field on `ui` (default off)
- `KeyG` ("Gate") toggles `portals`, sets the new pair on the
  current `game` (or sets a flag for the next `start()`)
- `#play-portal` card in `index.html` (purple/indigo `#a29bfe`)
- `#portal-pill` HUD pill
- `togglePortals()` mirrors `toggleObstacles()` shape
- New factory: `u.start(modeNormal)` calls `u.game =
  NewGameWithPortals(cols, rows, seed, obsCount, portals)`
  when `u.portals` is true
- `?withportals=1` URL flag for headless probes — boots the
  game with portals enabled on the first round (mirrors
  `?openlb=1`)

### `index.html`

- Add `KeyG` to the menu footer hint and the in-game hint
- Add `G` to the in-game restart hint
- Add CSS for `.card.portal` (purple) and `.portal-pill`

## Tests (Go)

1. `TestPickPortalPairDeterministic` — same seed → same pair;
   both cells inside the board, distinct, off the snake, off the
   food, distance ≥ 4.
2. `TestPickPortalPairRespectsBlocks` — block the board so only
   one valid pair remains; verify that pair is returned.
3. `TestPickPortalPairNoPair` — fully blocked board → returns
   `(zero, false)`.
4. `TestPickPortalPairIgnoresFoodAndHead` — head + food never
   in the pair; head's 3×3 neighborhood also excluded.
5. `TestPortalTeleport` — `Portal{A,B}.Partner(A) == B`,
   `Partner(B) == A`, `Partner(other) == other`.
6. `TestPortalAtAndEnabled` — `At(A) && At(B)`;
   `Enabled()` true for valid, false for `{-1,-1},{-1,-1}`.
7. `TestStepTeleportsHead` — small board, head enters A, assert
   head lands on B and body shifted correctly.
8. `TestStepPortalIntoSelfDies` — body wraps around B; head on
   A; step → `dead == true`.
9. `TestStepPortalIntoObstacleDies` — enable obstacles, place B
   under an obstacle; step → `dead == true`.
10. `TestDecideFindsPathThroughPortal` — engineer a layout where
    the only path to food is via a portal jump; assert the AI's
    chosen direction is the one that enters the portal.
11. `TestPortalsComposesWithWrap` — both on; head on A in the
    corner; assert next-step is B (not a wrap-shifted version of
    A).
12. `TestPortalsComposesWithObstacles` — placement excludes
    obstacles (covered by TestPickPortalPairRespectsBlocks with
    an obstacle-blocked board).

## Verification (browser)

- `bash build.sh dist` (rebuilds wasm + dist)
- `python -m http.server 8765 --directory dist`
- Real-Chrome screenshot at
  `http://127.0.0.1:8765/?afterg=1` — menu should now show 6
  cards: NORMAL, DEMO, WRAP, WALLS, LEADERBOARD, PORTAL. Footer
  hint includes `G`.
- `?withportals=1` — game starts with portals enabled, two `◉`
  glyphs visible on the board, snake advance through one portal
  teleports to the other.
- `?openlb=1` — leaderboard still works, no regression to
  cycle 3.
- `?obstacles=1&withportals=1` — both modes compose; verify two
  glyphs avoid the obstacle cells.

## Out of scope for cycle 4

- TS / Rust / C / Python / Elm ports of portal mode. Each is
  its own cycle; they port **from** the Go reference.
- Multi-portal (>2 glyphs) chains. Two is the canonical mode.
- Portal speed boost or wrap-immune scoring. Vanilla teleport only.
- Visual flash / particle burst on teleport. Polish for a later
  cycle if requested.
- AI benchmark JSON re-run with portals enabled. Out of scope;
  the benchmark covers the no-modifier default.
