//go:build js && wasm

// Snake for the browser, compiled with GOOS=js GOARCH=wasm.
//
// Unlike the Rust and C builds in this repo — which export a board and let
// JavaScript paint it — this one owns the whole thing: Go reaches through
// syscall/js to read input, drive requestAnimationFrame, update the HUD and
// draw to the canvas. The page ships no game logic at all.
package main

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"syscall/js"
	"time"
)

const (
	cellPx  = 24              // logical px per cell; CSS scales the canvas
	tickMS  = 130.0           // ms per move, matching the TypeScript engine
	bestKey = "snake-go-best" // localStorage
	badgeMS = 2500.0          // how long the tier badge stays up
)

type mode int

const (
	modeNormal mode = iota
	modeDemo
)

// tierStyle is the label and colour for each AI tier.
var tierStyle = [3]struct{ label, color string }{
	{"Tier 1 · Safe Pursuit", "#00cec9"},
	{"Tier 2 · Max Space", "#fdcb6e"},
	{"Fallback · Toward Food", "#ff7675"},
}

type particle struct {
	x, y, vx, vy, life, max float64
}

type ui struct {
	doc    js.Value
	canvas js.Value
	ctx    js.Value

	menuScreen, gameScreen        js.Value
	scoreEl, bestEl, menuBestEl   js.Value
	menuAIEl, gameOverEl, badgeEl js.Value
	visionBtn, dpad, hintEl       js.Value
	wrapBtn, wrapPill             js.Value
	obsBtn, obsPill               js.Value
	lbBtn, lbPanel, lbRows        js.Value

	game *Game
	mode mode

	running, dead, paused bool
	wrap, obstacles       bool
	acc, last             float64
	best                  int

	showVision bool
	lastTier   int
	badgeUntil float64
	decision   Decision

	particles []particle
	foodPop   float64
	headPulse float64
	lastFood  Point

	touchX, touchY float64
	touchFired     bool
	holds          []js.Func // keep callbacks alive
}

// toggleWrap flips the wall-wrap mode and re-arms the HUD pill. Safe to call
// from the menu or in-game.
func (u *ui) toggleWrap() {
	u.wrap = !u.wrap
	if u.game != nil {
		u.game.SetWrap(u.wrap)
	}
	if u.wrapPill.IsUndefined() || u.wrapPill.IsNull() {
		return
	}
	if u.wrap {
		u.wrapPill.Set("textContent", "WRAP")
		u.wrapPill.Set("hidden", false)
	} else {
		u.wrapPill.Set("hidden", true)
	}
}

// toggleObstacles flips the static-obstacles mode. The current round is
// kept — the new layout is only applied on the next start().
func (u *ui) toggleObstacles() {
	u.obstacles = !u.obstacles
	if u.obsPill.IsUndefined() || u.obsPill.IsNull() {
		return
	}
	if u.obstacles {
		u.obsPill.Set("textContent", "WALLS")
		u.obsPill.Set("hidden", false)
	} else {
		u.obsPill.Set("hidden", true)
	}
}

// obsCount is the per-round number of obstacles for the chosen board.
// Scales with board area so the density stays roughly constant.
func (u *ui) obsCount() int {
	cols, rows := Cols, Rows
	if u.portrait() {
		cols, rows = ColsPortrait, RowsPortrait
	}
	n := (cols * rows) / 16
	if n < 8 {
		n = 8
	}
	if n > 60 {
		n = 60
	}
	return n
}

func main() {
	doc := js.Global().Get("document")
	u := &ui{
		doc:        doc,
		canvas:     doc.Call("getElementById", "board"),
		menuScreen: doc.Call("getElementById", "menu"),
		gameScreen: doc.Call("getElementById", "game"),
		scoreEl:    doc.Call("getElementById", "score"),
		bestEl:     doc.Call("getElementById", "best"),
		menuBestEl: doc.Call("getElementById", "menu-best"),
		menuAIEl:   doc.Call("getElementById", "menu-ai"),
		gameOverEl: doc.Call("getElementById", "gameover"),
		badgeEl:    doc.Call("getElementById", "tier-badge"),
		visionBtn:  doc.Call("getElementById", "vision-btn"),
		dpad:       doc.Call("getElementById", "dpad"),
		hintEl:     doc.Call("getElementById", "controls-hint"),
		wrapBtn:    doc.Call("getElementById", "play-wrap"),
		wrapPill:   doc.Call("getElementById", "wrap-pill"),
		obsBtn:     doc.Call("getElementById", "play-obstacles"),
		obsPill:    doc.Call("getElementById", "obstacles-pill"),
		lbBtn:      doc.Call("getElementById", "play-leaderboard"),
		lbPanel:    doc.Call("getElementById", "leaderboard"),
		lbRows:     doc.Call("getElementById", "leaderboard-rows"),
	}
	u.ctx = u.canvas.Call("getContext", "2d")
	u.best = u.loadBest()
	u.showBest()
	u.loadBenchmark()
	if js.Global().Get("location").Get("search").String() == "?openlb=1" {
		u.showLeaderboard()
	}
	u.bind()
	u.loop()

	select {} // keep the Go runtime alive for the callbacks
}

func (u *ui) on(target js.Value, event string, fn func(js.Value)) {
	cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
		var e js.Value
		if len(args) > 0 {
			e = args[0]
		}
		fn(e)
		return nil
	})
	u.holds = append(u.holds, cb)
	target.Call("addEventListener", event, cb)
}

func (u *ui) bind() {
	u.on(u.doc.Call("getElementById", "play-normal"), "click", func(js.Value) { u.start(modeNormal) })
	u.on(u.doc.Call("getElementById", "play-demo"), "click", func(js.Value) { u.start(modeDemo) })
	u.on(u.wrapBtn, "click", func(js.Value) { u.toggleWrap() })
	u.on(u.obsBtn, "click", func(js.Value) { u.toggleObstacles() })
	u.on(u.lbBtn, "click", func(js.Value) { u.showLeaderboard() })
	u.on(u.visionBtn, "click", func(js.Value) { u.toggleVision() })

	keys := map[string]Point{
		"ArrowUp": Up, "KeyW": Up,
		"ArrowRight": Right, "KeyD": Right,
		"ArrowDown": Down, "KeyS": Down,
		"ArrowLeft": Left, "KeyA": Left,
	}

	u.on(js.Global(), "keydown", func(e js.Value) {
		code := e.Get("code").String()

		if !u.menuScreen.Get("hidden").Bool() {
			switch code {
			case "KeyN":
				u.start(modeNormal)
			case "KeyD":
				u.start(modeDemo)
			case "KeyT":
				u.toggleWrap()
			case "KeyO":
				u.toggleObstacles()
			case "KeyL":
				u.showLeaderboard()
			}
			return
		}
		if d, ok := keys[code]; ok && u.mode == modeNormal {
			e.Call("preventDefault")
			if !u.dead {
				u.game.QueueDir(d)
			}
			return
		}
		switch code {
		case "Space", "Enter":
			e.Call("preventDefault")
			u.restartIfDead()
		case "KeyM":
			u.openMenu()
		case "KeyV":
			u.toggleVision()
		case "KeyT":
			u.toggleWrap()
		case "KeyO":
			u.toggleObstacles()
		case "KeyL":
			u.showLeaderboard()
		case "Escape":
			if !u.lbPanel.Get("hidden").Bool() {
				u.hideLeaderboard()
			} else {
				u.openMenu()
			}
		case "KeyP":
			if !u.dead {
				u.paused = !u.paused
			}
		}
	})
	touchStart := js.FuncOf(func(_ js.Value, args []js.Value) any {
		t := args[0].Get("touches").Index(0)
		u.touchX, u.touchY = t.Get("clientX").Float(), t.Get("clientY").Float()
		u.touchFired = false
		return nil
	})
	stage := u.doc.Call("getElementById", "stage")
	passive := map[string]any{"passive": true}
	u.holds = append(u.holds, touchStart)
	stage.Call("addEventListener", "touchstart", touchStart, passive)

	touchMove := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if u.touchFired || u.mode != modeNormal || u.dead || !u.running {
			return nil
		}
		t := args[0].Get("touches").Index(0)
		dx := t.Get("clientX").Float() - u.touchX
		dy := t.Get("clientY").Float() - u.touchY
		if math.Hypot(dx, dy) < 24 {
			return nil
		}
		u.touchFired = true
		switch {
		case math.Abs(dx) > math.Abs(dy) && dx > 0:
			u.game.QueueDir(Right)
		case math.Abs(dx) > math.Abs(dy):
			u.game.QueueDir(Left)
		case dy > 0:
			u.game.QueueDir(Down)
		default:
			u.game.QueueDir(Up)
		}
		return nil
	})
	u.holds = append(u.holds, touchMove)
	stage.Call("addEventListener", "touchmove", touchMove, passive)

	touchEnd := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		if !u.touchFired {
			u.restartIfDead()
		}
		return nil
	})
	u.holds = append(u.holds, touchEnd)
	stage.Call("addEventListener", "touchend", touchEnd, passive)

	u.on(stage, "click", func(js.Value) { u.restartIfDead() })

	buttons := u.dpad.Call("querySelectorAll", "button")
	for i := 0; i < buttons.Length(); i++ {
		btn := buttons.Index(i)
		// dataset values arrive as strings.
		n, err := strconv.Atoi(btn.Get("dataset").Get("dir").String())
		if err != nil {
			continue
		}
		dir := Dirs[n]
		u.on(btn, "pointerdown", func(e js.Value) {
			e.Call("preventDefault")
			if !u.dead && u.running {
				u.game.QueueDir(dir)
			}
		})
	}
}

// showLeaderboard opens the leaderboard overlay and renders the 68
// NPC entries plus the live player best-score row. Cheap to call.
func (u *ui) showLeaderboard() {
	if u.lbPanel.IsUndefined() || u.lbPanel.IsNull() {
		return
	}
	rows := lbGetLeaderboard()
	doc := js.Global().Get("document")
	frag := doc.Call("createDocumentFragment")
	for _, r := range rows {
		div := doc.Call("createElement", "div")
		cls := "lb-row"
		if r.IsPlayer {
			cls += " lb-player"
		}
		div.Set("className", cls)
		div.Set("textContent", FormatRow(r))
		frag.Call("appendChild", div)
	}
	u.lbRows.Set("innerHTML", "")
	u.lbRows.Call("appendChild", frag)
	u.lbPanel.Set("hidden", false)
}

func (u *ui) hideLeaderboard() {
	if u.lbPanel.IsUndefined() || u.lbPanel.IsNull() {
		return
	}
	u.lbPanel.Set("hidden", true)
}

func (u *ui) portrait() bool {
	return js.Global().Get("innerHeight").Int() > js.Global().Get("innerWidth").Int()
}

func (u *ui) coarsePointer() bool {
	return js.Global().Call("matchMedia", "(pointer: coarse)").Get("matches").Bool()
}

func (u *ui) openMenu() {
	u.running = false
	u.gameScreen.Set("hidden", true)
	u.menuScreen.Set("hidden", false)
	u.showBest()
}

func (u *ui) start(m mode) {
	u.mode = m
	u.menuScreen.Set("hidden", true)
	u.gameScreen.Set("hidden", false)

	// Board shape follows the viewport, then stays put for the round.
	cols, rows := Cols, Rows
	if u.portrait() {
		cols, rows = ColsPortrait, RowsPortrait
	}
	u.canvas.Set("width", cols*cellPx)
	u.canvas.Set("height", rows*cellPx)
	u.canvas.Get("style").Set("aspectRatio", fmt.Sprintf("%d / %d", cols, rows))

	obs := 0
	if u.obstacles {
		obs = u.obsCount()
	}
	u.game = NewGameWithObstacles(cols, rows, time.Now().UnixNano()+rand.Int63(), obs)
	u.game.SetWrap(u.wrap)
	u.dead = false
	u.paused = false
	u.running = true
	u.acc = 0
	u.particles = nil
	u.foodPop = 1
	u.headPulse = 1
	if u.game.food != nil {
		u.lastFood = *u.game.food
	}
	u.showVision = false
	u.lastTier = -1
	u.decision = Decision{}
	u.badgeEl.Get("classList").Call("remove", "show")

	u.scoreEl.Set("textContent", "Score: 0")
	u.gameOverEl.Set("hidden", true)
	u.visionBtn.Set("hidden", m != modeDemo)
	u.visionBtn.Set("textContent", "AI Vision: off")
	u.visionBtn.Get("classList").Call("remove", "on")
	u.dpad.Set("hidden", !(m == modeNormal && u.coarsePointer() && !u.portrait()))
	if m == modeDemo {
		u.hintEl.Set("innerHTML", "<kbd>V</kbd> AI vision &middot; <kbd>T</kbd> wrap &middot; <kbd>O</kbd> walls &middot; <kbd>M</kbd> menu &middot; <kbd>P</kbd> pause")
	} else {
		u.hintEl.Set("innerHTML", "<kbd>&larr;&uarr;&darr;&rarr;</kbd> / <kbd>WASD</kbd> move &middot; <kbd>T</kbd> wrap &middot; <kbd>O</kbd> walls &middot; <kbd>M</kbd> menu &middot; <kbd>P</kbd> pause &middot; swipe on mobile")
	}
	if u.wrap {
		u.wrapPill.Set("textContent", "WRAP")
		u.wrapPill.Set("hidden", false)
	} else {
		u.wrapPill.Set("hidden", true)
	}
	if u.obstacles {
		u.obsPill.Set("textContent", fmt.Sprintf("WALLS %d", u.game.ObstacleCount()))
		u.obsPill.Set("hidden", false)
	} else {
		u.obsPill.Set("hidden", true)
	}
}

func (u *ui) restartIfDead() {
	if u.dead {
		u.start(u.mode)
	}
}

func (u *ui) toggleVision() {
	if u.mode != modeDemo || !u.running {
		return
	}
	u.showVision = !u.showVision
	label := "AI Vision: off"
	if u.showVision {
		label = "AI Vision: on"
	}
	u.visionBtn.Set("textContent", label)
	if u.showVision {
		u.visionBtn.Get("classList").Call("add", "on")
	} else {
		u.visionBtn.Get("classList").Call("remove", "on")
		u.lastTier = -1
		u.badgeEl.Get("classList").Call("remove", "show")
	}
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

func (u *ui) showBest() {
	label := "Best: —"
	if u.best > 0 {
		label = "Best: " + strconv.Itoa(u.best)
	}
	u.bestEl.Set("textContent", label)
	u.menuBestEl.Set("textContent", label)
}

// loadBenchmark shows the same AI statistics the TypeScript menu does.
func (u *ui) loadBenchmark() {
	then := js.FuncOf(func(_ js.Value, args []js.Value) any {
		bm := args[0]
		if bm.Truthy() && bm.Get("runs").Int() > 0 {
			u.menuAIEl.Set("textContent", fmt.Sprintf("AI best: %d · win rate: %.0f%%",
				bm.Get("max").Int(), bm.Get("winRate").Float()*100))
		}
		return nil
	})
	u.holds = append(u.holds, then)
	parse := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if !args[0].Get("ok").Bool() {
			return js.Null()
		}
		return args[0].Call("json")
	})
	u.holds = append(u.holds, parse)
	js.Global().Call("fetch", "benchmark.json").
		Call("then", parse).Call("then", then)
}

func (u *ui) loop() {
	var frame js.Func
	frame = js.FuncOf(func(_ js.Value, args []js.Value) any {
		js.Global().Call("requestAnimationFrame", frame)
		now := args[0].Float()
		dt := math.Min(now-u.last, 100)
		u.last = now
		if !u.running {
			return nil
		}

		if !u.dead && !u.paused {
			u.acc += dt
			if u.acc >= tickMS {
				u.acc = 0
				u.advance()
			}
		}

		// Tweens run every frame regardless of the game clock.
		u.foodPop = math.Min(1, u.foodPop+dt/200)
		u.headPulse += (1 - u.headPulse) * math.Min(1, dt/90)
		alive := u.particles[:0]
		for _, p := range u.particles {
			p.life -= dt
			p.x += p.vx * dt * 0.001
			p.y += p.vy * dt * 0.001
			if p.life > 0 {
				alive = append(alive, p)
			}
		}
		u.particles = alive

		if u.badgeUntil > 0 && now > u.badgeUntil {
			u.badgeUntil = 0
			u.badgeEl.Get("classList").Call("remove", "show")
		}

		u.draw()
		return nil
	})
	u.holds = append(u.holds, frame)
	js.Global().Call("requestAnimationFrame", frame)
}

func (u *ui) advance() {
	if u.mode == modeDemo {
		u.decision = Decide(u.game)
		u.game.ForceDir(u.decision.Dir)
		if u.showVision {
			u.showTier(int(u.decision.Tier))
		}
	}

	ate, died, won := u.game.Step()
	u.scoreEl.Set("textContent", "Score: "+strconv.Itoa(u.game.score))

	if ate {
		u.pulse(u.scoreEl)
		u.headPulse = 1.15
		u.spawnParticles()
	}
	if u.game.food != nil && *u.game.food != u.lastFood {
		u.lastFood = *u.game.food
		u.foodPop = 0
	}

	if died || won {
		u.dead = true
		if u.game.score > u.best {
			u.best = u.game.score
			js.Global().Get("localStorage").Call("setItem", bestKey, strconv.Itoa(u.best))
		}
		u.showBest()
		text := "Game Over\nTap to retry"
		if won {
			text = "You Win!\nTap to retry"
		}
		u.gameOverEl.Set("textContent", text)
		u.gameOverEl.Set("hidden", false)
		u.restartAnimation(u.gameOverEl)
		u.badgeEl.Get("classList").Call("remove", "show")
		if died && !won {
			u.canvas.Get("classList").Call("remove", "shake")
			u.canvas.Get("offsetWidth")
			u.canvas.Get("classList").Call("add", "shake")
		}
	}
}

func (u *ui) pulse(el js.Value) {
	el.Get("classList").Call("remove", "pop")
	el.Get("offsetWidth")
	el.Get("classList").Call("add", "pop")
}

func (u *ui) restartAnimation(el js.Value) {
	el.Get("style").Set("animation", "none")
	el.Get("offsetWidth")
	el.Get("style").Set("animation", "")
}

func (u *ui) showTier(tier int) {
	if tier == u.lastTier {
		return
	}
	u.lastTier = tier
	u.badgeEl.Set("textContent", tierStyle[tier].label)
	u.badgeEl.Get("style").Set("color", tierStyle[tier].color)
	u.badgeEl.Get("classList").Call("add", "show")
	u.badgeUntil = u.last + badgeMS
}

func (u *ui) spawnParticles() {
	head := u.game.snake[0]
	cx := float64(head.X*cellPx) + cellPx/2
	cy := float64(head.Y*cellPx) + cellPx/2
	for i := 0; i < 20; i++ {
		angle := rand.Float64() * 2 * math.Pi
		speed := 40 + rand.Float64()*100
		u.particles = append(u.particles, particle{
			x: cx, y: cy,
			vx:   math.Cos(angle) * speed,
			vy:   math.Sin(angle) * speed,
			life: 300 + rand.Float64()*300,
			max:  600,
		})
	}
}

func (u *ui) draw() {
	ctx := u.ctx
	g := u.game
	w := float64(g.cols * cellPx)
	h := float64(g.rows * cellPx)

	alpha := 1.0
	if u.dead {
		alpha = 0.4
	}

	ctx.Set("fillStyle", "#1a1a2e")
	ctx.Call("fillRect", 0, 0, w, h)

	ctx.Set("strokeStyle", "rgba(22, 33, 62, 0.55)")
	ctx.Set("lineWidth", 1)
	ctx.Call("beginPath")
	for x := 0; x <= g.cols; x++ {
		ctx.Call("moveTo", float64(x*cellPx)+0.5, 0)
		ctx.Call("lineTo", float64(x*cellPx)+0.5, h)
	}
	for y := 0; y <= g.rows; y++ {
		ctx.Call("moveTo", 0, float64(y*cellPx)+0.5)
		ctx.Call("lineTo", w, float64(y*cellPx)+0.5)
	}
	ctx.Call("stroke")

	if u.mode == modeDemo && u.showVision {
		u.drawVision()
	}

	ctx.Set("globalAlpha", alpha)

	// Body.
	ctx.Set("fillStyle", "#6c5ce7")
	for _, p := range g.snake[1:] {
		ctx.Call("fillRect", float64(p.X*cellPx)+1, float64(p.Y*cellPx)+1, cellPx-2, cellPx-2)
	}

	// Food, with a pop-in as it spawns.
	if g.food != nil {
		ease := 1 - math.Pow(1-u.foodPop, 3)
		ctx.Set("fillStyle", "#ff7675")
		ctx.Call("beginPath")
		ctx.Call("arc",
			float64(g.food.X*cellPx)+cellPx/2, float64(g.food.Y*cellPx)+cellPx/2,
			math.Max(0.5, (cellPx/2-2)*ease), 0, 2*math.Pi)
		ctx.Call("fill")
	}

	// Head: glow, body, direction-aware eyes.
	head := g.snake[0]
	hx := float64(head.X*cellPx) + cellPx/2
	hy := float64(head.Y*cellPx) + cellPx/2
	size := (cellPx - 1) * u.headPulse

	if !u.dead {
		ctx.Set("globalAlpha", alpha*0.18)
		ctx.Set("fillStyle", "#00cec9")
		ctx.Call("fillRect", hx-(cellPx+6)/2, hy-(cellPx+6)/2, cellPx+6, cellPx+6)
		ctx.Set("globalAlpha", alpha)
	}

	ctx.Set("fillStyle", "#00cec9")
	ctx.Call("fillRect", hx-size/2, hy-size/2, size, size)

	if !u.dead {
		off := cellPx * 0.22
		var eyes [2][2]float64
		switch g.dir {
		case Up:
			eyes = [2][2]float64{{-off, -off}, {off, -off}}
		case Down:
			eyes = [2][2]float64{{-off, off}, {off, off}}
		case Left:
			eyes = [2][2]float64{{-off, -off}, {-off, off}}
		default:
			eyes = [2][2]float64{{off, -off}, {off, off}}
		}
		ctx.Set("fillStyle", "#0a0a1e")
		for _, e := range eyes {
			ctx.Call("beginPath")
			ctx.Call("arc", hx+e[0], hy+e[1], math.Max(1.5, cellPx*0.12), 0, 2*math.Pi)
			ctx.Call("fill")
		}
	}

	ctx.Set("globalAlpha", 1)

	// Eat particles.
	for _, p := range u.particles {
		t := math.Max(0, p.life/p.max)
		ctx.Set("globalAlpha", t)
		ctx.Set("fillStyle", "#ff7675")
		ctx.Call("beginPath")
		ctx.Call("arc", p.x, p.y, 4*t, 0, 2*math.Pi)
		ctx.Call("fill")
	}
	ctx.Set("globalAlpha", 1)

	if u.paused && !u.dead {
		ctx.Set("fillStyle", "rgba(26, 26, 46, 0.7)")
		ctx.Call("fillRect", 0, 0, w, h)
		ctx.Set("fillStyle", "#00cec9")
		ctx.Set("font", "bold 22px ui-monospace, monospace")
		ctx.Set("textAlign", "center")
		ctx.Call("fillText", "PAUSED", w/2, h/2)
	}
}

// drawVision shows the AI's reasoning: reachable region shaded, path dashed.
func (u *ui) drawVision() {
	ctx := u.ctx

	ctx.Set("fillStyle", "rgba(108, 92, 231, 0.15)")
	for _, c := range u.decision.Reachable {
		ctx.Call("fillRect", float64(c.X*cellPx)+1, float64(c.Y*cellPx)+1, cellPx-2, cellPx-2)
	}

	path := u.decision.Path
	if len(path) < 2 {
		return
	}
	ctx.Set("strokeStyle", "rgba(255, 118, 117, 0.9)")
	ctx.Set("lineWidth", 2)
	ctx.Call("beginPath")
	for i := 1; i < len(path); i++ {
		x1 := float64(path[i-1].X*cellPx) + cellPx/2
		y1 := float64(path[i-1].Y*cellPx) + cellPx/2
		x2 := float64(path[i].X*cellPx) + cellPx/2
		y2 := float64(path[i].Y*cellPx) + cellPx/2
		dx, dy := x2-x1, y2-y1
		steps := int(math.Hypot(dx, dy) / 6)
		if steps < 1 {
			ctx.Call("moveTo", x1, y1)
			ctx.Call("lineTo", x2, y2)
			continue
		}
		for s := 0; s < steps; s += 2 {
			f := float64(s) / float64(steps)
			t := float64(s+1) / float64(steps)
			ctx.Call("moveTo", x1+f*dx, y1+f*dy)
			ctx.Call("lineTo", x1+t*dx, y1+t*dy)
		}
	}
	ctx.Call("stroke")
}
