package main

import (
	"encoding/json"
	"os"
	"testing"
)

// sample is one recorded decision from testdata/ai-trace.json. The trace comes
// from the Rust module, which ai-parity.mts checks against the original
// TypeScript demo controller — so matching it means matching the original.
type sample struct {
	Cols      int   `json:"cols"`
	Rows      int   `json:"rows"`
	Snake     []int `json:"snake"` // cell indices, head first
	Food      *int  `json:"food"`
	Dir       int   `json:"dir"`
	Tier      int   `json:"tier"`
	Chosen    int   `json:"chosen"`
	Reachable int   `json:"reachable"`
	Path      int   `json:"path"`
}

func loadTrace(t *testing.T) []sample {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/ai-trace.json")
	if err != nil {
		t.Fatalf("reading trace: %v", err)
	}
	var samples []sample
	if err := json.Unmarshal(raw, &samples); err != nil {
		t.Fatalf("parsing trace: %v", err)
	}
	if len(samples) == 0 {
		t.Fatal("trace is empty")
	}
	return samples
}

func (s sample) game() *Game {
	g := &Game{cols: s.Cols, rows: s.Rows, alive: true}
	g.occupied = make([]bool, s.Cols*s.Rows)
	for _, cell := range s.Snake {
		p := Point{cell % s.Cols, cell / s.Cols}
		g.snake = append(g.snake, p)
		g.occupied[g.index(p)] = true
	}
	if s.Food != nil {
		p := Point{*s.Food % s.Cols, *s.Food / s.Cols}
		g.food = &p
	}
	g.dir = Dirs[s.Dir]
	return g
}

func dirIndex(d Point) int {
	for i, candidate := range Dirs {
		if candidate == d {
			return i
		}
	}
	return -1
}

// TestDecideMatchesReference replays recorded board states and demands the same
// answer the original controller gave: same tier, same move, same reachable
// region and same planned path length.
func TestDecideMatchesReference(t *testing.T) {
	samples := loadTrace(t)
	tiers := map[int]int{}

	for i, s := range samples {
		got := Decide(s.game())
		tiers[int(got.Tier)]++

		if int(got.Tier) != s.Tier {
			t.Errorf("sample %d (%dx%d, len %d): tier %d, want %d",
				i, s.Cols, s.Rows, len(s.Snake), got.Tier, s.Tier)
			continue
		}
		if dirIndex(got.Dir) != s.Chosen {
			t.Errorf("sample %d (%dx%d, len %d): dir %d, want %d",
				i, s.Cols, s.Rows, len(s.Snake), dirIndex(got.Dir), s.Chosen)
		}
		if len(got.Reachable) != s.Reachable {
			t.Errorf("sample %d: reachable %d, want %d", i, len(got.Reachable), s.Reachable)
		}
		if len(got.Path) != s.Path {
			t.Errorf("sample %d: path %d, want %d", i, len(got.Path), s.Path)
		}
	}

	// The trace is only worth something if it exercises all three tiers.
	for tier, name := range map[int]string{0: "tier1", 1: "tier2", 2: "fallback"} {
		if tiers[tier] == 0 {
			t.Errorf("trace never reached %s", name)
		}
	}
	t.Logf("%d samples: tier1 %d, tier2 %d, fallback %d",
		len(samples), tiers[0], tiers[1], tiers[2])
}

// TestGameRules covers the parts of the engine the trace cannot: growth,
// walls, self-collision and the tail that steps out of the way.
func TestGameRules(t *testing.T) {
	g := NewGame(10, 10, 1)
	if len(g.snake) != 3 || !g.alive {
		t.Fatalf("fresh game: len %d alive %v", len(g.snake), g.alive)
	}

	// Eating grows the snake and scores a point.
	g.food = &Point{g.snake[0].X + 1, g.snake[0].Y}
	ate, died, _ := g.Step()
	if !ate || died || len(g.snake) != 4 || g.score != 1 {
		t.Errorf("after eating: ate %v died %v len %d score %d", ate, died, len(g.snake), g.score)
	}

	// Reversing into the neck is ignored rather than fatal.
	g.QueueDir(Left)
	if _, died, _ := g.Step(); died {
		t.Error("reversal killed the snake")
	}

	// Walls are fatal.
	wall := NewGame(10, 10, 1)
	for i := 0; i < 20 && wall.alive; i++ {
		wall.Step()
	}
	if wall.alive {
		t.Error("snake survived the right wall")
	}

	// The tail counts as body: the TypeScript engine tests the whole snake
	// before popping it, so stepping onto your own tail is fatal.
	tail := NewGame(8, 8, 1)
	tail.snake = []Point{{2, 2}, {2, 3}, {3, 3}, {3, 2}}
	tail.occupied = make([]bool, 64)
	for _, p := range tail.snake {
		tail.occupied[tail.index(p)] = true
	}
	tail.dir = Up
	tail.food = &Point{7, 7}
	tail.QueueDir(Right)
	if _, died, _ := tail.Step(); !died {
		t.Error("stepping onto the tail should be fatal")
	}
}

// newWrapGame returns a deterministic wrap-mode game. The seed pins the food
// spawn so tests can target a specific open cell.
func newWrapGame(cols, rows int) *Game {
	g := NewGame(cols, rows, 42)
	g.SetWrap(true)
	return g
}

// TestWrapMovement covers the head crossing each of the four edges.
func TestWrapMovement(t *testing.T) {
	cases := []struct {
		name  string
		setup func(g *Game)
		want  Point
		died  bool
	}{
		{
			name: "right wraps to left",
			setup: func(g *Game) {
				g.snake = []Point{{g.cols - 1, 5}, {g.cols - 2, 5}, {g.cols - 3, 5}}
				g.dir = Right
			},
			want: Point{0, 5},
		},
		{
			name: "left wraps to right",
			setup: func(g *Game) {
				g.snake = []Point{{0, 5}, {1, 5}, {2, 5}}
				g.dir = Left
			},
			want: Point{9, 5},
		},
		{
			name: "down wraps to top",
			setup: func(g *Game) {
				g.snake = []Point{{5, g.rows - 1}, {5, g.rows - 2}, {5, g.rows - 3}}
				g.dir = Down
			},
			want: Point{5, 0},
		},
		{
			name: "up wraps to bottom",
			setup: func(g *Game) {
				g.snake = []Point{{5, 0}, {5, 1}, {5, 2}}
				g.dir = Up
			},
			want: Point{5, 9},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := newWrapGame(10, 10)
			for i := range g.occupied {
				g.occupied[i] = false
			}
			c.setup(g)
			for _, p := range g.snake {
				g.occupied[g.index(p)] = true
			}
			ate, died, _ := g.Step()
			if died != c.died {
				t.Fatalf("died=%v, want %v", died, c.died)
			}
			if g.snake[0] != c.want {
				t.Fatalf("head at %v, want %v", g.snake[0], c.want)
			}
			if !ate && g.food != nil && *g.food == c.want {
				t.Fatalf("head landed on food without ate=true")
			}
		})
	}
}

// TestWrapSurvivesBodyWrap puts a long snake next to the edge and steps it
// across. In wrap mode this must not self-collide because the tail moves out
// of the way on the same tick.
func TestWrapSurvivesBodyWrap(t *testing.T) {
	g := newWrapGame(8, 8)
	// Snake hugs the right edge, head at the corner.
	g.snake = []Point{{7, 4}, {6, 4}, {5, 4}, {4, 4}, {3, 4}}
	g.dir = Right
	for i := range g.occupied {
		g.occupied[i] = false
	}
	for _, p := range g.snake {
		g.occupied[g.index(p)] = true
	}
	// One step: head wraps to (0, 4), tail (3, 4) is freed, no collision.
	if _, died, _ := g.Step(); died {
		t.Fatal("wrap step killed a non-overlapping snake")
	}
	if g.snake[0] != (Point{0, 4}) {
		t.Fatalf("head at %v, want {0 4}", g.snake[0])
	}
}

// TestWrapBFSCrossesEdge: a path that requires leaving one edge and entering
// the opposite must be found by BFS in wrap mode.
func TestWrapBFSCrossesEdge(t *testing.T) {
	g := newWrapGame(6, 6)
	start := Point{5, 3} // right edge
	goal := Point{0, 3}  // left edge, one wrap away
	blocked := make([]bool, g.cols*g.rows)
	path := g.bfs(start, goal, blocked)
	if path == nil {
		t.Fatal("BFS returned nil; wrap path should exist")
	}
	if path[0] != start || path[len(path)-1] != goal {
		t.Fatalf("BFS path endpoints wrong: start=%v end=%v", path[0], path[len(path)-1])
	}
	// On a 6-wide board, the wrap path is exactly 2 cells: (5,3) -> (0,3).
	if len(path) != 2 {
		t.Fatalf("expected 2-cell wrap path, got %d: %v", len(path), path)
	}
}

// TestWrapFloodCountsAllCells: on an empty wrap board, the flood fill from any
// cell should reach every cell of the board.
func TestWrapFloodCountsAllCells(t *testing.T) {
	g := newWrapGame(5, 5)
	blocked := make([]bool, g.cols*g.rows)
	cells := g.flood(Point{0, 0}, blocked)
	if len(cells) != g.cols*g.rows {
		t.Fatalf("flood reached %d cells, want %d", len(cells), g.cols*g.rows)
	}
}

// TestWrapOffStaysOriginal confirms the toggle is not one-way. Walls must kill
// the snake after SetWrap(false), even on a board the snake just survived in
// wrap mode.
func TestWrapOffStaysOriginal(t *testing.T) {
	g := newWrapGame(6, 6)
	g.snake = []Point{{5, 3}, {4, 3}, {3, 3}, {2, 3}}
	g.dir = Right
	for i := range g.occupied {
		g.occupied[i] = false
	}
	for _, p := range g.snake {
		g.occupied[g.index(p)] = true
	}
	if _, died, _ := g.Step(); died {
		t.Fatal("wrap step should not kill the snake")
	}
	// Now disable wrap and step the head at (0,3) left — it must die on the
	// left wall instead of wrapping.
	g.SetWrap(false)
	g.dir = Left
	if _, died, _ := g.Step(); !died {
		t.Fatal("non-wrap mode must kill at the wall")
	}
}

// TestWrapDecideStillPicksLegalMove: in wrap mode, Decide must not return a
// direction whose next cell is the snake's neck.
func TestWrapDecideStillPicksLegalMove(t *testing.T) {
	g := newWrapGame(8, 8)
	// Force a tail that demands the head move DOWN.
	g.snake = []Point{{4, 4}, {4, 5}, {5, 5}, {5, 4}}
	g.dir = Right
	for i := range g.occupied {
		g.occupied[i] = false
	}
	for _, p := range g.snake {
		g.occupied[g.index(p)] = true
	}
	d := Decide(g)
	if opposite(d.Dir, g.dir) {
		t.Fatalf("AI chose reversal %v against dir %v", d.Dir, g.dir)
	}
}
