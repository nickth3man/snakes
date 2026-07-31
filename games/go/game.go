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
	obstacles  []bool // static walls placed at game start; do not move
	food       *Point
	dir        Point
	queue      []Point // up to two pending player directions
	score      int
	alive      bool
	won        bool
	wrap       bool // when true, the board is a torus: leaving one edge enters the opposite
	rng        *rand.Rand
}

func NewGame(cols, rows int, seed int64) *Game {
	return NewGameWithObstacles(cols, rows, seed, 0)
}

// NewGameWithObstacles creates a fresh game and, if n > 0, places n static
// obstacles. The seed is shared between obstacle placement and food spawn
// so the same seed reproduces the same board.
func NewGameWithObstacles(cols, rows int, seed int64, n int) *Game {
	g := &Game{cols: cols, rows: rows, rng: rand.New(rand.NewSource(seed))}
	g.obstacles = make([]bool, cols*rows)
	g.Start()
	if n > 0 {
		g.PlaceObstacles(n)
	}
	return g
}

func (g *Game) SetWrap(wrap bool) { g.wrap = wrap }

// SetWrap toggles torus mode. Movement, BFS, flood fill, and the wall-adjacency
// heuristic all read this flag. The board shape does not change.

// Wrap reports whether torus mode is on. Exposed for the UI and for tests.
func (g *Game) Wrap() bool { return g.wrap }

// wrapPoint folds a coordinate back into the board. Used in wrap mode to
// realise "leaving one edge enters the opposite" without the caller having
// to know about the board size.
func (g *Game) wrapPoint(p Point) Point {
	x := ((p.X % g.cols) + g.cols) % g.cols
	y := ((p.Y % g.rows) + g.rows) % g.rows
	return Point{x, y}
}

// stepFrom returns the cell the head lands on after moving one step in d. In
// wrap mode the cell is always on the board; in wall mode an out-of-bounds
// step returns ok=false so the caller can declare it a death.
func (g *Game) stepFrom(p Point, d Point) (next Point, ok bool) {
	next = p.add(d)
	if g.wrap {
		return g.wrapPoint(next), true
	}
	if !g.inBounds(next) {
		return Point{}, false
	}
	return next, true
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

// obstacleAt is true for cells that hold a static wall. Obstacles are
// parallel to occupied but are not cleared as the snake moves.
func (g *Game) obstacleAt(p Point) bool { return g.obstacles[g.index(p)] }

// blockedAt is the union of snake body and static obstacles. Both
// movement (Step) and the AI (bfs / flood) consult this to keep
// collision logic in one place.
func (g *Game) blockedAt(p Point) bool {
	return g.occupiedAt(p) || g.obstacleAt(p)
}

// PlaceObstacles fills up to n cells with static walls, never on the
// snake, the food, the outer edge, or the head's first step. The
// board is left unchanged if there is not enough room for n obstacles.
func (g *Game) PlaceObstacles(n int) {
	if n <= 0 {
		return
	}
	// Reserve the cells the snake would die on immediately: the head
	// and the four neighbours it can reach on the first tick.
	reserved := make([]bool, g.cols*g.rows)
	for _, p := range g.snake {
		reserved[g.index(p)] = true
	}
	if g.food != nil {
		reserved[g.index(*g.food)] = true
	}
	if head := g.snake[0]; len(g.snake) > 0 {
		for _, d := range Dirs {
			nb, ok := g.stepFrom(head, d)
			if !ok {
				continue
			}
			reserved[g.index(nb)] = true
		}
	}
	// Walk the board, skipping reserved cells and the outer edge.
	candidates := make([]int, 0, g.cols*g.rows)
	for i := 0; i < g.cols*g.rows; i++ {
		if reserved[i] {
			continue
		}
		p := Point{i % g.cols, i / g.cols}
		if p.X == 0 || p.Y == 0 || p.X == g.cols-1 || p.Y == g.rows-1 {
			continue
		}
		candidates = append(candidates, i)
	}
	if len(candidates) == 0 {
		return
	}
	// Fisher-Yates with the existing RNG so the same seed reproduces.
	for i := len(candidates) - 1; i > 0; i-- {
		j := g.rng.Intn(i + 1)
		candidates[i], candidates[j] = candidates[j], candidates[i]
	}
	for i := 0; i < n && i < len(candidates); i++ {
		g.obstacles[candidates[i]] = true
	}
}

// ObstacleCount returns the number of static walls currently on the
// board. Exposed for the HUD and for tests.
func (g *Game) ObstacleCount() int {
	n := 0
	for _, b := range g.obstacles {
		if b {
			n++
		}
	}
	return n
}

// spawnFood picks uniformly among cells that are neither snake body nor
// static obstacle. A board that is full of snake plus obstacles is a win.
func (g *Game) spawnFood() {
	free := g.cols*g.rows - len(g.snake) - g.ObstacleCount()
	if free <= 0 {
		g.food = nil
		g.alive = false
		g.won = true
		return
	}
	nth := g.rng.Intn(free)
	for i := 0; i < len(g.occupied); i++ {
		if g.occupied[i] || g.obstacles[i] {
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

	next, ok := g.stepFrom(g.snake[0], g.dir)
	if !ok {
		g.alive = false
		return false, true, false
	}
	// Static obstacles are a hard wall, checked before the body so an
	// obstacle under the tail never gets a free pass from the tail
	// vacating on the same tick.
	if g.obstacleAt(next) {
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
