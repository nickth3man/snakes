# Feature Plan: Leaderboard (Port from TypeScript — Go first)

## Brainstormed candidates (5)

1. **Leaderboard** — port the existing TypeScript leaderboard to Go
   first, then the other 5 languages in follow-up cycles.
2. **Power-ups** — random spawn of speed / slow-mo / ghost /
   score-mult that change gameplay for ~10s.
3. **Sound effects** — Web Audio API eat / die / win chimes.
4. **Time-attack mode** — countdown timer; each food adds time.
5. **Two-player split-screen** — second snake on the same board,
   controlled by WASD.

## Choice: Leaderboard (Go first)

Why this one next:

- **Real parity gap.** TypeScript ships a 431-line leaderboard
  (core + store + panel + names). Rust, Go, C, Python, Elm have
  none. The repo README explicitly says "every version is the
  same game, feature for feature"; the leaderboard breaks that
  promise today.
- **Pure logic, no rendering debt.** The Go version paints its own
  canvas through `syscall/js`, but the leaderboard can ship as DOM
  text in the existing `#menu` overlay. No new canvas work, no new
  draw loop, no new input handler beyond an `L` shortcut.
- **Deterministic test surface.** Score curve + NPC generation both
  use a fixed seed and a fixed RNG. Every output is reproducible,
  which means golden-file parity testing against the TypeScript
  source is straightforward.
- **Trivial to test in `go test`.** No DOM, no localStorage, no JS
  interop in the core — just `mulberry32`, arithmetic, and string
  formatting. Same pattern as `game.go` and `ai.go`.
- **Independent of cycles 1 and 2.** Leaderboard reads the player's
  best score (`snake-go-best` localStorage key); it does not touch
  `wrap`, `obstacles`, BFS, or flood fill. Zero regression risk on
  the AI parity test.

## Scope (this cycle: Go only)

- Port `leaderboard-core.ts` → `leaderboard.go` (pure logic).
- Port `leaderboard-store.ts` → `leaderboard_store.go` (localStorage
  IO; only the file that imports `syscall/js`).
- Wire a leaderboard panel into the existing menu DOM using DOM
  text only (no canvas work).
- Add an `L` key shortcut to toggle the panel.
- Follow-up cycles (out of scope here): Rust, C, Python, Elm ports
  each get their own cycle, all driven by the same golden fixture.

## Design

### `leaderboard.go` (new file, pure logic, no DOM)

Lives alongside `game.go` and `ai.go` so the rules stay in plain
Go. No `js && wasm` build tag, no `syscall/js` import. Compiles
under `go test` on the host.

#### Constants (mirroring TypeScript)

```go
const (
    ScoreMax        = 320
    ScoreMin        = 8
    CurveExponent   = 0.55
    JitterAmp       = 7
    LeaderboardSeed = 0x514d3a75
    NameCount       = 68
    NameDisplayMax  = 22
)
```

#### Types

```go
type NpcEntry struct {
    Rank  int    // 1..68, curated power rank
    Name  string // never truncated
    Score int
}

type PlayerEntry struct {
    Score int
}

type LeaderboardRow struct {
    SortRank  int
    Score     int
    IsPlayer  bool
    Name      string
    PowerRank int // 0 for player
}
```

#### Pure functions

- `ScoreCurveBase(rank1 int) int`
- `GenerateNpcEntries(seed uint32) []NpcEntry`
- `MergeWithPlayer(npc []NpcEntry, p *PlayerEntry) []LeaderboardRow`
- `TruncateName(name string) string`
- `FormatRow(row LeaderboardRow) string`

The score curve, RNG, names list, and row format must match the
TypeScript output byte-for-byte so the same golden fixture drives
the other 5 ports.

### `leaderboard_store.go` (new file, syscall/js only)

The localStorage boundary is the only piece that needs
`syscall/js`. Split it out so the pure core stays host-testable.

- `LoadNpcEntries(js.Global()) []NpcEntry` — reads
  `snake-leaderboard` from localStorage, parses JSON, validates
  `version == 1 && seed == LeaderboardSeed && len(entries) == 68`,
  regenerates if any check fails, persists the new doc.
- `ReadBestScore(js.Global()) int` — reads `snake-go-best` (the
  key already owned by `main.go`); returns 0 on miss/parse error.
- `GetLeaderboard(js.Global()) []LeaderboardRow` — pure composition.

### `main.go` (UI)

- New `ui` fields: `leaderboardEl js.Value`, `leaderboardRows
  js.Value`.
- `bind()` wires an `L` key shortcut (menu and in-game).
- On `L`: call `GetLeaderboard`, write each row into
  `leaderboardRows`, toggle `leaderboardEl.hidden`.
- Panel hides on game start and on `M`.

### `index.html`

- New `#leaderboard` `<div>` inside `#menu`, hidden by default.
- Rows go in `#leaderboard-rows`; player row gets a class for cyan.
- CSS for monospace rows; player row in cyan accent.
- `L` added to the footer hint and to the in-game controls hint.

## Tests

- `TestScoreCurveBase` — monotonic descending; rank 1 ≥ 300,
  rank 68 ≤ 12.
- `TestScoreCurveBaseEndpoints` — exactly `ScoreMax` and
  `ScoreMin` ± `JitterAmp` at the boundaries.
- `TestGenerateNpcEntriesDeterministic` — same seed → same 68
  scores; byte-equal between two calls.
- `TestGenerateNpcEntriesLength` — exactly `NameCount` rows.
- `TestMergeWithPlayerNoPlayer` — `SortRank` equals 1..68 with no
  duplicate.
- `TestMergeWithPlayerHighScore` — best=500 → player row is
  SortRank 1, others shift down by one.
- `TestMergeWithPlayerTieBreaker` — score equal to NPC's → player
  appears above.
- `TestMergeWithPlayerZeroScore` — best=0 → no player row.
- `TestTruncateName` — short name unchanged; long name truncated
  with `…`.
- `TestFormatRowPlayer` — output matches the gold string.
- `TestFormatRowNPC` — output matches the gold string.
- `TestFormatRowAllRows` — every row has the exact format; no
  trailing whitespace; length ≤ 42.
- `TestGoldenParity` — generate NPC with `LeaderboardSeed`,
  compare every `formatRow` to a JSON fixture
  (`testdata/leaderboard-golden.json`) baked from the TypeScript
  output. One golden test, byte-equal, no skipping.
- `TestDecideMatchesReference` — must remain green.
- `TestGameRules` — must remain green.

## Verification

- `go test ./...` — all green, including the golden parity test.
- `GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w"` —
  clean.
- `bash build.sh dist` — clean.
- Real headless Chrome screenshot of the menu with leaderboard
  shown: rows render, player row is cyan, "YOU" appears once.
- After playing a game and exceeding an NPC's score, refresh and
  re-screenshot: the player row moves up.
- Server log shows `GET /snake.wasm`, `GET /wasm_exec.js`, and
  `GET /leaderboard-golden.json` (if served separately) all 200.

## Out of scope (this cycle)

- Porting leaderboard to Rust, C, Python, Elm. Each gets its own
  follow-up cycle driven by the same golden fixture.
- Animated row entrance, scroll inertia, hover sound. The panel is
  pure DOM text; visual flair can come later.
- Multi-profile support, cloud sync, server-side scores.
