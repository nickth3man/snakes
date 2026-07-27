package main

import "math/rand"

// Point is a board cell. The origin is the top-left corner.
type Point struct{ X, Y int }

// Directions in UP, RIGHT, DOWN, LEFT order. The order decides ties in the
// demo AI, so it is load-bearing.
var (
	Up    = Point{0, -1}
	Right = Point{1, 0}
	Down  = Point{0, 1}
	Left  = Point{-1, 0}
	Dirs  = [4]Point{Up, Right, Down, Left}
)

func (p Point) add(d Point) Point { return Point{p.X + d.X, p.Y + d.Y} }

func opposite(a, b Point) bool { return a.X == -b.X && a.Y == -b.Y }

// Board sizes, chosen by orientation the way the TypeScript scene does.
const (
	Cols         = 30
	Rows         = 22
	ColsPortrait = 18
	RowsPortrait = 38
)

// Game is the rules, and nothing else: no DOM, no drawing, no timing.
type Game struct {
	cols, rows int
	snake      []Point // head first
	occupied   []bool
	food       *Point
	dir        Point
	queue      []Point // up to two pending player directions
	score      int
	alive      bool
	won        bool
	rng        *rand.Rand
}

func NewGame(cols, rows int, seed int64) *Game {
	g := &Game{cols: cols, rows: rows, rng: rand.New(rand.NewSource(seed))}
	g.Start()
	return g
}

// Start deals the opening position: three cells, mid-row, facing right.
func (g *Game) Start() {
	mid := g.rows / 2
	startCol := min(g.cols/2, 5)
	safe := max(startCol, 2)

	g.snake = []Point{{safe, mid}, {safe - 1, mid}, {safe - 2, mid}}
	g.occupied = make([]bool, g.cols*g.rows)
	for _, p := range g.snake {
		g.occupied[g.index(p)] = true
	}
	g.dir = Right
	g.queue = g.queue[:0]
	g.score = 0
	g.alive = true
	g.won = false
	g.spawnFood()
}

func (g *Game) index(p Point) int { return p.Y*g.cols + p.X }

func (g *Game) inBounds(p Point) bool {
	return p.X >= 0 && p.Y >= 0 && p.X < g.cols && p.Y < g.rows
}

func (g *Game) occupiedAt(p Point) bool { return g.occupied[g.index(p)] }

// spawnFood picks uniformly among empty cells; a full board is a win.
func (g *Game) spawnFood() {
	free := g.cols*g.rows - len(g.snake)
	if free <= 0 {
		g.food = nil
		g.alive = false
		g.won = true
		return
	}
	nth := g.rng.Intn(free)
	for i, taken := range g.occupied {
		if taken {
			continue
		}
		if nth == 0 {
			p := Point{i % g.cols, i / g.cols}
			g.food = &p
			return
		}
		nth--
	}
}

// QueueDir buffers a player direction, at most two deep, refusing reversals.
func (g *Game) QueueDir(d Point) {
	if len(g.queue) >= 2 {
		return
	}
	last := g.dir
	if n := len(g.queue); n > 0 {
		last = g.queue[n-1]
	}
	if opposite(d, last) {
		return
	}
	g.queue = append(g.queue, d)
}

// ForceDir replaces the queue outright. The demo AI decides one move at a
// time, so its choice should not sit behind stale player input.
func (g *Game) ForceDir(d Point) {
	g.queue = append(g.queue[:0], d)
}

// Step advances one move and reports what happened.
func (g *Game) Step() (ate, died, won bool) {
	if !g.alive {
		return false, true, g.won
	}

	want := g.dir
	if len(g.queue) > 0 {
		want = g.queue[0]
		g.queue = g.queue[1:]
	}
	// A reversal would drive the head straight into the neck; ignore it.
	if !(len(g.snake) > 1 && opposite(want, g.dir)) {
		g.dir = want
	}

	next := g.snake[0].add(g.dir)
	if !g.inBounds(next) {
		g.alive = false
		return false, true, false
	}
	// Self-collision is checked against the whole body, tail included — the
	// TypeScript engine does the same before the tail is popped.
	if g.occupiedAt(next) {
		g.alive = false
		return false, true, false
	}

	ate = g.food != nil && next == *g.food
	if !ate {
		tail := g.snake[len(g.snake)-1]
		g.occupied[g.index(tail)] = false
		g.snake = g.snake[:len(g.snake)-1]
	}

	g.snake = append([]Point{next}, g.snake...)
	g.occupied[g.index(next)] = true

	if ate {
		g.score++
		g.spawnFood()
	}
	return ate, false, g.won
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
