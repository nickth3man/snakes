// Snake for the browser, compiled with GOOS=js GOARCH=wasm.
//
// Unlike the Rust and C builds in this repo — which export a grid and let
// JavaScript paint it — this one owns the whole thing: Go reaches through
// syscall/js to read input, drive requestAnimationFrame and draw to the canvas.
package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"syscall/js"
)

const (
	cols     = 24
	rows     = 24
	cell     = 24
	baseTick = 120.0 // ms between moves at length 3
	minTick  = 65.0  // floor, reached after ~28 apples
	bestKey  = "snake-go-best"
)

type point struct{ x, y int }

var (
	up    = point{0, -1}
	right = point{1, 0}
	down  = point{0, 1}
	left  = point{-1, 0}
)

// game is pure logic: no DOM, no rendering, easy to reason about.
type game struct {
	snake    []point // head first
	occupied [cols * rows]bool
	food     point
	dir      point
	pending  point
	score    int
	alive    bool
	won      bool
}

func newGame() *game {
	g := &game{}
	g.reset()
	return g
}

func (g *game) reset() {
	g.snake = []point{{4, rows / 2}, {3, rows / 2}, {2, rows / 2}}
	g.occupied = [cols * rows]bool{}
	for _, p := range g.snake {
		g.occupied[p.y*cols+p.x] = true
	}
	g.dir, g.pending = right, right
	g.score = 0
	g.alive = true
	g.won = false
	g.spawnFood()
}

// spawnFood picks uniformly among empty cells.
func (g *game) spawnFood() {
	free := cols*rows - len(g.snake)
	if free <= 0 {
		return
	}
	nth := rand.Intn(free)
	for i := range g.occupied {
		if g.occupied[i] {
			continue
		}
		if nth == 0 {
			g.food = point{i % cols, i / cols}
			return
		}
		nth--
	}
}

// turn queues a direction change, ignoring reversals into the neck.
func (g *game) turn(d point) {
	if len(g.snake) > 1 && d.x == -g.dir.x && d.y == -g.dir.y {
		return
	}
	g.pending = d
}

// step advances one tick and reports whether the snake ate this tick.
func (g *game) step() (ate bool) {
	if !g.alive {
		return false
	}
	g.dir = g.pending

	head := g.snake[0]
	next := point{head.x + g.dir.x, head.y + g.dir.y}

	if next.x < 0 || next.y < 0 || next.x >= cols || next.y >= rows {
		g.alive = false
		return false
	}

	ate = next == g.food
	tail := g.snake[len(g.snake)-1]
	// Chasing your own tail is fine: it moves out of the way this tick.
	if g.occupied[next.y*cols+next.x] && !(next == tail && !ate) {
		g.alive = false
		return false
	}

	if !ate {
		g.occupied[tail.y*cols+tail.x] = false
		g.snake = g.snake[:len(g.snake)-1]
	}

	g.snake = append([]point{next}, g.snake...)
	g.occupied[next.y*cols+next.x] = true

	if ate {
		g.score += 10
		if len(g.snake) == cols*rows {
			g.alive = false
			g.won = true
			return true
		}
		g.spawnFood()
	}
	return ate
}

// tickMS is the current move interval, shortening as the snake grows.
func (g *game) tickMS() float64 {
	t := baseTick - 2*float64(g.score/10)
	if t < minTick {
		return minTick
	}
	return t
}

type ui struct {
	game    *game
	ctx     js.Value
	overlay js.Value
	title   js.Value
	text    js.Value
	scoreEl js.Value
	bestEl  js.Value
	best    int
	running bool
	paused  bool
	acc     float64
	last    float64
}

func main() {
	doc := js.Global().Get("document")
	canvas := doc.Call("getElementById", "board")
	canvas.Set("width", cols*cell)
	canvas.Set("height", rows*cell)

	u := &ui{
		game:    newGame(),
		ctx:     canvas.Call("getContext", "2d"),
		overlay: doc.Call("getElementById", "overlay"),
		title:   doc.Call("getElementById", "overlay-title"),
		text:    doc.Call("getElementById", "overlay-text"),
		scoreEl: doc.Call("getElementById", "score"),
		bestEl:  doc.Call("getElementById", "best"),
	}
	u.best = u.loadBest()
	u.bestEl.Set("textContent", strconv.Itoa(u.best))

	u.bindKeys(doc)
	u.bindTouch(doc)

	u.draw()
	u.showOverlay("GO SNAKE", "Press <kbd>SPACE</kbd> or tap to start")
	u.loop()

	select {} // keep the Go runtime alive for the callbacks
}

func (u *ui) loadBest() int {
	v := js.Global().Get("localStorage").Call("getItem", bestKey)
	if v.IsNull() || v.IsUndefined() {
		return 0
	}
	n, err := strconv.Atoi(v.String())
	if err != nil {
		return 0
	}
	return n
}

func (u *ui) saveBest() {
	js.Global().Get("localStorage").Call("setItem", bestKey, strconv.Itoa(u.best))
}

func (u *ui) showOverlay(title, text string) {
	u.title.Set("textContent", title)
	u.text.Set("innerHTML", text)
	u.overlay.Set("hidden", false)
}

func (u *ui) newGame() {
	u.game.reset()
	u.acc = 0
	u.paused = false
	u.running = true
	u.scoreEl.Set("textContent", "0")
	u.overlay.Set("hidden", true)
	u.draw()
}

func (u *ui) loop() {
	var frame js.Func
	frame = js.FuncOf(func(_ js.Value, args []js.Value) any {
		js.Global().Call("requestAnimationFrame", frame)
		now := args[0].Float()
		if !u.running || u.paused {
			u.last = now
			return nil
		}
		u.acc += now - u.last
		u.last = now
		if u.acc < u.game.tickMS() {
			return nil
		}
		u.acc = 0

		u.game.step()
		u.scoreEl.Set("textContent", strconv.Itoa(u.game.score))
		if !u.game.alive {
			u.running = false
			if u.game.score > u.best {
				u.best = u.game.score
				u.saveBest()
				u.bestEl.Set("textContent", strconv.Itoa(u.best))
			}
			title := "GAME OVER"
			if u.game.won {
				title = "PERFECT GAME"
			}
			u.showOverlay(title, fmt.Sprintf(
				"Score <b>%d</b> &middot; length %d<br />Press <kbd>SPACE</kbd> or tap to play again",
				u.game.score, len(u.game.snake)))
		}
		u.draw()
		return nil
	})
	js.Global().Call("requestAnimationFrame", frame)
}

var keyDirs = map[string]point{
	"ArrowUp": up, "KeyW": up,
	"ArrowRight": right, "KeyD": right,
	"ArrowDown": down, "KeyS": down,
	"ArrowLeft": left, "KeyA": left,
}

func (u *ui) bindKeys(doc js.Value) {
	js.Global().Call("addEventListener", "keydown", js.FuncOf(func(_ js.Value, args []js.Value) any {
		e := args[0]
		code := e.Get("code").String()
		if d, ok := keyDirs[code]; ok {
			e.Call("preventDefault")
			if u.running && !u.paused {
				u.game.turn(d)
			} else if !u.paused {
				u.newGame()
			}
			return nil
		}
		switch code {
		case "Space", "Enter":
			e.Call("preventDefault")
			if !u.running {
				u.newGame()
			}
		case "KeyP":
			if u.running {
				u.paused = !u.paused
				if u.paused {
					u.showOverlay("PAUSED", "Press <kbd>P</kbd> to resume")
				} else {
					u.overlay.Set("hidden", true)
				}
			}
		}
		return nil
	}))
}

func (u *ui) bindTouch(doc js.Value) {
	stage := doc.Call("querySelector", ".stage")
	var startX, startY float64
	opts := map[string]any{"passive": true}

	stage.Call("addEventListener", "touchstart", js.FuncOf(func(_ js.Value, args []js.Value) any {
		t := args[0].Get("touches").Index(0)
		startX, startY = t.Get("clientX").Float(), t.Get("clientY").Float()
		return nil
	}), opts)

	stage.Call("addEventListener", "touchend", js.FuncOf(func(_ js.Value, args []js.Value) any {
		t := args[0].Get("changedTouches").Index(0)
		dx := t.Get("clientX").Float() - startX
		dy := t.Get("clientY").Float() - startY
		if !u.running {
			u.newGame()
			return nil
		}
		if dx*dx+dy*dy < 24*24 {
			return nil
		}
		switch {
		case abs(dx) > abs(dy) && dx > 0:
			u.game.turn(right)
		case abs(dx) > abs(dy):
			u.game.turn(left)
		case dy > 0:
			u.game.turn(down)
		default:
			u.game.turn(up)
		}
		return nil
	}), opts)

	stage.Call("addEventListener", "click", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		if !u.running && !u.paused {
			u.newGame()
		}
		return nil
	}))
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func (u *ui) draw() {
	ctx := u.ctx
	ctx.Set("fillStyle", "#1a1a2e")
	ctx.Call("fillRect", 0, 0, cols*cell, rows*cell)

	ctx.Set("strokeStyle", "#16213e")
	ctx.Set("lineWidth", 1)
	ctx.Call("beginPath")
	for i := 1; i < cols; i++ {
		ctx.Call("moveTo", float64(i*cell)+0.5, 0)
		ctx.Call("lineTo", float64(i*cell)+0.5, rows*cell)
	}
	for j := 1; j < rows; j++ {
		ctx.Call("moveTo", 0, float64(j*cell)+0.5)
		ctx.Call("lineTo", cols*cell, float64(j*cell)+0.5)
	}
	ctx.Call("stroke")

	// Food.
	ctx.Set("fillStyle", "#ff7675")
	ctx.Call("beginPath")
	ctx.Call("arc", float64(u.game.food.x*cell)+cell/2, float64(u.game.food.y*cell)+cell/2, cell*0.32, 0, 6.2832)
	ctx.Call("fill")

	// Body, then head on top.
	ctx.Set("fillStyle", "#6c5ce7")
	for _, p := range u.game.snake[1:] {
		roundRect(ctx, float64(p.x*cell)+2, float64(p.y*cell)+2, cell-4, cell-4, 5)
	}

	head := u.game.snake[0]
	hx, hy := float64(head.x*cell), float64(head.y*cell)
	ctx.Set("fillStyle", "#00cec9")
	roundRect(ctx, hx+1, hy+1, cell-2, cell-2, 6)
	ctx.Set("fillStyle", "#0a0a1e")
	ctx.Call("beginPath")
	ctx.Call("arc", hx+cell*0.35, hy+cell*0.38, cell*0.09, 0, 6.2832)
	ctx.Call("arc", hx+cell*0.65, hy+cell*0.38, cell*0.09, 0, 6.2832)
	ctx.Call("fill")
}

func roundRect(ctx js.Value, x, y, w, h, r float64) {
	ctx.Call("beginPath")
	ctx.Call("roundRect", x, y, w, h, r)
	ctx.Call("fill")
}
