package main

// The demo-mode AI, a direct port of the TypeScript demo controller.
//
// Tier 1: safe food pursuit — a move that can reach the food by BFS while
// leaving at least 1.2x the snake's length of reachable cells. Ties go to the
// shorter path, then more wall-hugging, then more turns, all for the show.
//
// Tier 2: maximise open space — the move with the largest flood fill. Ties go
// to wall-adjacent cells, then to whatever lands nearer the food.
//
// Fallback: the legal move that lands nearest the food.

type Tier int

const (
	Tier1 Tier = iota
	Tier2
	Fallback
)

// Decision is the chosen move plus everything the AI-vision overlay draws.
type Decision struct {
	Dir       Point
	Tier      Tier
	Path      []Point // from the current head to the food, inclusive
	Reachable []Point
}

type move struct {
	dir  Point
	next Point
}

// Decide picks the AI's next direction and explains itself.
func Decide(g *Game) Decision {
	head := g.snake[0]

	var moves []move
	for _, d := range Dirs {
		if len(g.snake) > 1 && opposite(d, g.dir) {
			continue
		}
		next := head.add(d)
		if !g.inBounds(next) || g.occupiedAt(next) {
			continue
		}
		moves = append(moves, move{d, next})
	}

	// After one step the tail moves on, so it is not an obstacle for lookahead.
	blocked := make([]bool, g.cols*g.rows)
	for i := 0; i < len(g.snake)-1; i++ {
		blocked[g.index(g.snake[i])] = true
	}

	if len(moves) == 0 {
		return Decision{Dir: g.dir, Tier: Fallback}
	}
	if len(moves) == 1 {
		return Decision{
			Dir:       moves[0].dir,
			Tier:      Fallback,
			Reachable: g.flood(moves[0].next, blocked),
		}
	}

	if g.food != nil {
		bestIdx, bestLen, bestHugs, bestTurns := -1, 0, 0, 0
		for i, m := range moves {
			path := g.bfs(m.next, *g.food, blocked)
			if path == nil {
				continue
			}
			space := len(g.flood(m.next, blocked))
			// space < len(snake) * 1.2, in integers.
			if space*5 < len(g.snake)*6 {
				continue
			}
			hugs := g.wallHugs(path)
			turns := countTurns(path)
			better := bestIdx < 0 || len(path) < bestLen ||
				(len(path) == bestLen &&
					(hugs > bestHugs || (hugs == bestHugs && turns > bestTurns)))
			if better {
				bestIdx, bestLen, bestHugs, bestTurns = i, len(path), hugs, turns
			}
		}

		if bestIdx >= 0 {
			chosen := moves[bestIdx]
			// The overlay draws from the current head, so prepend it.
			path := append([]Point{head}, g.bfs(chosen.next, *g.food, blocked)...)
			return Decision{
				Dir:       chosen.dir,
				Tier:      Tier1,
				Path:      path,
				Reachable: g.flood(chosen.next, blocked),
			}
		}
	}

	{
		bestIdx, bestSpace, bestHug, bestDist := -1, 0, 0, 0
		for i, m := range moves {
			space := len(g.flood(m.next, blocked))
			hug := 0
			if g.wallAdjacent(m.next) {
				hug = 1
			}
			dist := 0
			if g.food != nil {
				dist = abs(g.food.X-m.next.X) + abs(g.food.Y-m.next.Y)
			}
			better := bestIdx < 0 || space > bestSpace ||
				(space == bestSpace &&
					(hug > bestHug || (hug == bestHug && dist < bestDist)))
			if better {
				bestIdx, bestSpace, bestHug, bestDist = i, space, hug, dist
			}
		}

		if bestIdx >= 0 && bestSpace > 1 {
			chosen := moves[bestIdx]
			return Decision{
				Dir:       chosen.dir,
				Tier:      Tier2,
				Reachable: g.flood(chosen.next, blocked),
			}
		}
	}

	chosen := moves[0]
	if g.food != nil {
		bestDist := 1 << 30
		for _, m := range moves {
			dist := abs(g.food.X-m.next.X) + abs(g.food.Y-m.next.Y)
			if dist < bestDist {
				bestDist, chosen = dist, m
			}
		}
	}
	return Decision{
		Dir:       chosen.dir,
		Tier:      Fallback,
		Reachable: g.flood(chosen.next, blocked),
	}
}

// bfs returns the shortest path from start to goal inclusive, or nil.
func (g *Game) bfs(start, goal Point, blocked []bool) []Point {
	if blocked[g.index(start)] {
		return nil
	}
	if start == goal {
		return []Point{start}
	}

	seen := make([]bool, g.cols*g.rows)
	parent := make([]int, g.cols*g.rows)
	for i := range parent {
		parent[i] = -1
	}
	seen[g.index(start)] = true
	queue := []Point{start}

	for len(queue) > 0 {
		pos := queue[0]
		queue = queue[1:]
		for _, d := range Dirs {
			next := pos.add(d)
			if !g.inBounds(next) {
				continue
			}
			ni := g.index(next)
			if blocked[ni] || seen[ni] {
				continue
			}
			parent[ni] = g.index(pos)
			if next == goal {
				// Walk the parents back, then reverse.
				var path []Point
				for i := ni; ; i = parent[i] {
					path = append(path, Point{i % g.cols, i / g.cols})
					if i == g.index(start) {
						break
					}
				}
				for l, r := 0, len(path)-1; l < r; l, r = l+1, r-1 {
					path[l], path[r] = path[r], path[l]
				}
				return path
			}
			seen[ni] = true
			queue = append(queue, next)
		}
	}

	return nil
}

// flood returns every cell reachable from start without crossing an obstacle.
func (g *Game) flood(start Point, blocked []bool) []Point {
	if blocked[g.index(start)] {
		return nil
	}
	seen := make([]bool, g.cols*g.rows)
	seen[g.index(start)] = true
	cells := []Point{start}

	for i := 0; i < len(cells); i++ {
		pos := cells[i]
		for _, d := range Dirs {
			next := pos.add(d)
			if !g.inBounds(next) {
				continue
			}
			ni := g.index(next)
			if blocked[ni] || seen[ni] {
				continue
			}
			seen[ni] = true
			cells = append(cells, next)
		}
	}

	return cells
}

func (g *Game) wallAdjacent(p Point) bool {
	return p.X == 0 || p.Y == 0 || p.X == g.cols-1 || p.Y == g.rows-1
}

func (g *Game) wallHugs(path []Point) int {
	n := 0
	for _, p := range path {
		if g.wallAdjacent(p) {
			n++
		}
	}
	return n
}

func countTurns(path []Point) int {
	if len(path) < 3 {
		return 0
	}
	turns := 0
	prev := Point{path[1].X - path[0].X, path[1].Y - path[0].Y}
	for i := 2; i < len(path); i++ {
		d := Point{path[i].X - path[i-1].X, path[i].Y - path[i-1].Y}
		if d != prev {
			turns++
			prev = d
		}
	}
	return turns
}
