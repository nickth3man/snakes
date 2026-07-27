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
