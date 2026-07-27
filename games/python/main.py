"""Snake in Python, played in the browser.

Pyodide is CPython compiled to WebAssembly, so this is the real interpreter --
dataclasses, ``collections.deque`` and all -- reaching out to the DOM through
the ``js`` bridge. Nothing here is transpiled to JavaScript.

Both game modes live here. Normal mode consumes a two-deep queue of player
directions; demo mode runs the three-tier AI in :func:`decide`, which also
reports its reasoning so the page can draw an "AI vision" overlay.
"""

from __future__ import annotations

import asyncio
import json
import math
import random
from collections import deque
from dataclasses import dataclass, field

from js import document, localStorage, window
from pyodide.ffi import create_proxy
from pyodide.http import pyfetch

Cell = tuple[int, int]

# Board sizes, chosen by orientation the way the TypeScript scene does.
COLS, ROWS = 30, 22
COLS_PORTRAIT, ROWS_PORTRAIT = 18, 38

CELL = 24  # logical px per cell; CSS scales the canvas
TICK = 130.0  # ms per move, matching the TypeScript engine
BADGE_MS = 2500.0
BEST_KEY = "snake-python-best"

BACKGROUND = "#1a1a2e"
GRID = "rgba(22, 33, 62, 0.55)"
HEAD_COLOR = "#00cec9"
BODY_COLOR = "#6c5ce7"
FOOD_COLOR = "#ff7675"
EYE_COLOR = "#0a0a1e"

UP = (0, -1)
RIGHT = (1, 0)
DOWN = (0, 1)
LEFT = (-1, 0)
# UP, RIGHT, DOWN, LEFT — the order decides ties in the AI, so it matters.
DIRS = (UP, RIGHT, DOWN, LEFT)

KEY_DIRS = {
    "ArrowUp": UP, "KeyW": UP,
    "ArrowRight": RIGHT, "KeyD": RIGHT,
    "ArrowDown": DOWN, "KeyS": DOWN,
    "ArrowLeft": LEFT, "KeyA": LEFT,
}

TIER1, TIER2, FALLBACK = 0, 1, 2
TIER_STYLE = (
    ("Tier 1 · Safe Pursuit", "#00cec9"),
    ("Tier 2 · Max Space", "#fdcb6e"),
    ("Fallback · Toward Food", "#ff7675"),
)


def opposite(a: Cell, b: Cell) -> bool:
    return a[0] == -b[0] and a[1] == -b[1]


@dataclass
class Game:
    """The rules, and nothing else: no DOM, no drawing, no timing."""

    cols: int = COLS
    rows: int = ROWS
    snake: list[Cell] = field(default_factory=list)  # head first
    occupied: set[Cell] = field(default_factory=set)
    food: Cell | None = None
    direction: Cell = RIGHT
    queue: deque[Cell] = field(default_factory=lambda: deque(maxlen=2))
    score: int = 0
    alive: bool = False
    won: bool = False

    def start(self) -> None:
        """Deal the opening position: three cells, mid-row, facing right."""
        mid = self.rows // 2
        safe = max(min(self.cols // 2, 5), 2)
        self.snake = [(safe, mid), (safe - 1, mid), (safe - 2, mid)]
        self.occupied = set(self.snake)
        self.direction = RIGHT
        self.queue.clear()
        self.score = 0
        self.alive = True
        self.won = False
        self.spawn_food()

    def in_bounds(self, cell: Cell) -> bool:
        return 0 <= cell[0] < self.cols and 0 <= cell[1] < self.rows

    def spawn_food(self) -> None:
        """Pick uniformly among empty cells; a full board is a win."""
        free = [
            (x, y)
            for y in range(self.rows)
            for x in range(self.cols)
            if (x, y) not in self.occupied
        ]
        if not free:
            self.food = None
            self.alive = False
            self.won = True
            return
        self.food = random.choice(free)

    def queue_dir(self, direction: Cell) -> None:
        """Buffer a player direction, at most two deep, refusing reversals."""
        if len(self.queue) >= 2:
            return
        last = self.queue[-1] if self.queue else self.direction
        if opposite(direction, last):
            return
        self.queue.append(direction)

    def force_dir(self, direction: Cell) -> None:
        """Replace the queue outright — the demo AI decides one move at a time."""
        self.queue.clear()
        self.queue.append(direction)

    def step(self) -> tuple[bool, bool, bool]:
        """Advance one move. Returns (ate, died, won)."""
        if not self.alive:
            return False, True, self.won

        want = self.queue.popleft() if self.queue else self.direction
        # A reversal would drive the head straight into the neck; ignore it.
        if not (len(self.snake) > 1 and opposite(want, self.direction)):
            self.direction = want

        hx, hy = self.snake[0]
        head = (hx + self.direction[0], hy + self.direction[1])

        if not self.in_bounds(head):
            self.alive = False
            return False, True, False
        # Self-collision is checked against the whole body, tail included — the
        # TypeScript engine does the same before the tail is popped.
        if head in self.occupied:
            self.alive = False
            return False, True, False

        ate = head == self.food
        if not ate:
            tail = self.snake.pop()
            self.occupied.discard(tail)

        self.snake.insert(0, head)
        self.occupied.add(head)

        if ate:
            self.score += 1
            self.spawn_food()
        return ate, False, self.won


@dataclass
class Decision:
    """The chosen move plus everything the AI-vision overlay draws."""

    direction: Cell
    tier: int
    path: list[Cell] = field(default_factory=list)
    reachable: list[Cell] = field(default_factory=list)


def decide(game: Game) -> Decision:
    """Pick the AI's next direction, a direct port of the TypeScript controller.

    Tier 1: safe food pursuit -- a move that can reach the food by BFS while
    leaving at least 1.2x the snake's length of reachable cells. Ties go to the
    shorter path, then more wall-hugging, then more turns, all for the show.

    Tier 2: maximise open space -- the move with the largest flood fill. Ties go
    to wall-adjacent cells, then to whatever lands nearer the food.

    Fallback: the legal move that lands nearest the food.
    """
    head = game.snake[0]

    # Step 1: immediate moves that are not reversals, walls or body cells.
    moves: list[tuple[Cell, Cell]] = []
    for direction in DIRS:
        if len(game.snake) > 1 and opposite(direction, game.direction):
            continue
        nxt = (head[0] + direction[0], head[1] + direction[1])
        if not game.in_bounds(nxt) or nxt in game.occupied:
            continue
        moves.append((direction, nxt))

    # After one step the tail moves on, so it is not an obstacle for lookahead.
    blocked = set(game.snake[:-1])

    if not moves:
        return Decision(game.direction, FALLBACK)
    if len(moves) == 1:
        direction, nxt = moves[0]
        return Decision(direction, FALLBACK, reachable=flood(game, nxt, blocked))

    # Step 2: tier 1 — reach the food while keeping room to breathe.
    if game.food is not None:
        best = None
        for direction, nxt in moves:
            path = bfs(game, nxt, game.food, blocked)
            if path is None:
                continue
            space = len(flood(game, nxt, blocked))
            # space < len(snake) * 1.2, in integers.
            if space * 5 < len(game.snake) * 6:
                continue
            key = (len(path), -wall_hugs(game, path), -count_turns(path))
            if best is None or key < best[0]:
                best = (key, direction, nxt)

        if best is not None:
            _, direction, nxt = best
            # The overlay draws from the current head, so prepend it.
            path = [head, *(bfs(game, nxt, game.food, blocked) or [])]
            return Decision(direction, TIER1, path, flood(game, nxt, blocked))

    # Step 3: tier 2 — head for the most open space.
    best = None
    for direction, nxt in moves:
        space = len(flood(game, nxt, blocked))
        hug = 1 if wall_adjacent(game, nxt) else 0
        dist = manhattan(game.food, nxt) if game.food is not None else 0
        key = (-space, -hug, dist)
        if best is None or key < best[0]:
            best = (key, direction, nxt, space)

    if best is not None and best[3] > 1:
        _, direction, nxt, _ = best
        return Decision(direction, TIER2, reachable=flood(game, nxt, blocked))

    # Step 4: fallback — lunge at the food and hope.
    direction, nxt = moves[0]
    food = game.food
    if food is not None:
        direction, nxt = min(moves, key=lambda m: manhattan(food, m[1]))
    return Decision(direction, FALLBACK, reachable=flood(game, nxt, blocked))


def manhattan(a: Cell, b: Cell) -> int:
    return abs(a[0] - b[0]) + abs(a[1] - b[1])


def wall_adjacent(game: Game, cell: Cell) -> bool:
    x, y = cell
    return x == 0 or y == 0 or x == game.cols - 1 or y == game.rows - 1


def wall_hugs(game: Game, path: list[Cell]) -> int:
    return sum(1 for cell in path if wall_adjacent(game, cell))


def count_turns(path: list[Cell]) -> int:
    """Count direction changes along a path."""
    if len(path) < 3:
        return 0
    turns = 0
    prev = (path[1][0] - path[0][0], path[1][1] - path[0][1])
    for before, after in zip(path[1:], path[2:]):
        delta = (after[0] - before[0], after[1] - before[1])
        if delta != prev:
            turns += 1
            prev = delta
    return turns


def bfs(game: Game, start: Cell, goal: Cell, blocked: set[Cell]) -> list[Cell] | None:
    """Shortest path from start to goal inclusive, or None if unreachable."""
    if start in blocked:
        return None
    if start == goal:
        return [start]

    parent: dict[Cell, Cell] = {start: start}
    queue = deque([start])
    while queue:
        x, y = queue.popleft()
        for dx, dy in DIRS:
            nxt = (x + dx, y + dy)
            if not game.in_bounds(nxt) or nxt in blocked or nxt in parent:
                continue
            parent[nxt] = (x, y)
            if nxt == goal:
                path = [nxt]
                while path[-1] != start:
                    path.append(parent[path[-1]])
                path.reverse()
                return path
            queue.append(nxt)
    return None


def flood(game: Game, start: Cell, blocked: set[Cell]) -> list[Cell]:
    """Every cell reachable from start without crossing an obstacle."""
    if start in blocked:
        return []
    seen = {start}
    cells = [start]
    queue = deque([start])
    while queue:
        x, y = queue.popleft()
        for dx, dy in DIRS:
            nxt = (x + dx, y + dy)
            if not game.in_bounds(nxt) or nxt in blocked or nxt in seen:
                continue
            seen.add(nxt)
            cells.append(nxt)
            queue.append(nxt)
    return cells


class UI:
    """Wires the game to the canvas, the keyboard, the menu and the HUD."""

    def __init__(self) -> None:
        self.canvas = document.getElementById("board")
        self.ctx = self.canvas.getContext("2d")
        self.menu_screen = document.getElementById("menu")
        self.game_screen = document.getElementById("game")
        self.stage = document.getElementById("stage")
        self.score_el = document.getElementById("score")
        self.best_el = document.getElementById("best")
        self.menu_best_el = document.getElementById("menu-best")
        self.menu_ai_el = document.getElementById("menu-ai")
        self.game_over_el = document.getElementById("gameover")
        self.badge_el = document.getElementById("tier-badge")
        self.vision_btn = document.getElementById("vision-btn")
        self.dpad = document.getElementById("dpad")
        self.hint_el = document.getElementById("controls-hint")

        self.game = Game()
        self.mode = "normal"
        self.running = False
        self.dead = False
        self.paused = False
        self.acc = 0.0
        self.last = 0.0

        self.show_vision = False
        self.last_tier = -1
        self.badge_until = 0.0
        self.decision = Decision(RIGHT, FALLBACK)

        self.particles: list[dict[str, float]] = []
        self.food_pop = 1.0
        self.head_pulse = 1.0
        self.last_food: Cell | None = None
        self.touch: tuple[float, float] | None = None
        self.touch_fired = False

        self.best = int(localStorage.getItem(BEST_KEY) or 0)
        self.show_best()
        self.load_benchmark()

        # Proxies must outlive the call that registers them, so keep references.
        self._proxies: list = []
        self.bind()

    # -- wiring ------------------------------------------------------------

    def on(self, target, event: str, handler, **options) -> None:
        proxy = create_proxy(handler)
        self._proxies.append(proxy)
        if options:
            target.addEventListener(event, proxy, options)
        else:
            target.addEventListener(event, proxy)

    def bind(self) -> None:
        self.on(document.getElementById("play-normal"), "click",
                lambda _e: self.start("normal"))
        self.on(document.getElementById("play-demo"), "click",
                lambda _e: self.start("demo"))
        self.on(document.getElementById("menu-btn"), "click",
                lambda _e: self.open_menu())
        self.on(self.vision_btn, "click", lambda _e: self.toggle_vision())
        self.on(window, "keydown", self.on_key)
        self.on(self.stage, "touchstart", self.on_touch_start)
        self.on(self.stage, "touchmove", self.on_touch_move)
        self.on(self.stage, "touchend", self.on_touch_end)
        self.on(self.stage, "click", lambda _e: self.restart_if_dead())

        for button in self.dpad.querySelectorAll("button"):
            direction = DIRS[int(button.dataset.dir)]
            self.on(button, "pointerdown", self.dpad_handler(direction))

        self.frame_proxy = create_proxy(self.frame)
        self._proxies.append(self.frame_proxy)
        window.requestAnimationFrame(self.frame_proxy)

    def dpad_handler(self, direction: Cell):
        def handler(event) -> None:
            event.preventDefault()
            if self.running and not self.dead:
                self.game.queue_dir(direction)
        return handler

    # -- screens -----------------------------------------------------------

    @staticmethod
    def portrait() -> bool:
        return window.innerHeight > window.innerWidth

    @staticmethod
    def coarse_pointer() -> bool:
        return window.matchMedia("(pointer: coarse)").matches

    def open_menu(self) -> None:
        self.running = False
        self.game_screen.hidden = True
        self.menu_screen.hidden = False
        self.show_best()

    def start(self, mode: str) -> None:
        self.mode = mode
        self.menu_screen.hidden = True
        self.game_screen.hidden = False

        # Board shape follows the viewport, then stays put for the round.
        cols, rows = (COLS_PORTRAIT, ROWS_PORTRAIT) if self.portrait() else (COLS, ROWS)
        self.canvas.width = cols * CELL
        self.canvas.height = rows * CELL
        self.canvas.style.aspectRatio = f"{cols} / {rows}"

        self.game = Game(cols=cols, rows=rows)
        self.game.start()
        self.dead = False
        self.paused = False
        self.running = True
        self.acc = 0.0
        self.particles = []
        self.food_pop = 1.0
        self.head_pulse = 1.0
        self.last_food = self.game.food
        self.show_vision = False
        self.last_tier = -1
        self.decision = Decision(RIGHT, FALLBACK)
        self.badge_el.classList.remove("show")

        self.score_el.textContent = "Score: 0"
        self.game_over_el.hidden = True
        self.vision_btn.hidden = mode != "demo"
        self.vision_btn.textContent = "AI Vision: off"
        self.vision_btn.classList.remove("on")
        self.dpad.hidden = not (
            mode == "normal" and self.coarse_pointer() and not self.portrait()
        )
        self.hint_el.innerHTML = (
            "<kbd>V</kbd> AI vision &middot; <kbd>M</kbd> menu &middot; <kbd>P</kbd> pause"
            if mode == "demo"
            else "<kbd>&larr;&uarr;&darr;&rarr;</kbd> / <kbd>WASD</kbd> move &middot; "
                 "<kbd>M</kbd> menu &middot; <kbd>P</kbd> pause &middot; swipe on mobile"
        )
        self.show_best()

    def restart_if_dead(self) -> None:
        if self.dead:
            self.start(self.mode)

    def toggle_vision(self) -> None:
        if self.mode != "demo" or not self.running:
            return
        self.show_vision = not self.show_vision
        self.vision_btn.textContent = f"AI Vision: {'on' if self.show_vision else 'off'}"
        if self.show_vision:
            self.vision_btn.classList.add("on")
        else:
            self.vision_btn.classList.remove("on")
            self.last_tier = -1
            self.badge_el.classList.remove("show")

    # -- scores ------------------------------------------------------------

    def show_best(self) -> None:
        label = f"Best: {self.best}" if self.best > 0 else "Best: —"
        self.best_el.textContent = label
        self.menu_best_el.textContent = label

    def load_benchmark(self) -> None:
        """Show the same AI statistics the TypeScript menu does."""

        async def load() -> None:
            try:
                response = await pyfetch("benchmark.json")
                data = json.loads(await response.string())
            except Exception:
                self.menu_ai_el.textContent = " "
                return
            if data.get("runs", 0) > 0:
                self.menu_ai_el.textContent = (
                    f"AI best: {data['max']} · win rate: {data['winRate'] * 100:.0f}%"
                )
            else:
                self.menu_ai_el.textContent = " "

        asyncio.ensure_future(load())

    # -- input -------------------------------------------------------------

    def on_key(self, event) -> None:
        code = event.code

        if not self.menu_screen.hidden:
            if code == "KeyN":
                self.start("normal")
            elif code == "KeyD":
                self.start("demo")
            return

        if code in KEY_DIRS and self.mode == "normal":
            event.preventDefault()
            if not self.dead:
                self.game.queue_dir(KEY_DIRS[code])
            return

        if code in ("Space", "Enter"):
            event.preventDefault()
            self.restart_if_dead()
        elif code == "KeyM":
            self.open_menu()
        elif code == "KeyV":
            self.toggle_vision()
        elif code == "KeyP" and not self.dead:
            self.paused = not self.paused

    def on_touch_start(self, event) -> None:
        touch = event.touches.item(0)
        self.touch = (touch.clientX, touch.clientY)
        self.touch_fired = False

    def on_touch_move(self, event) -> None:
        if self.touch is None or self.touch_fired:
            return
        if self.mode != "normal" or self.dead or not self.running:
            return
        touch = event.touches.item(0)
        dx = touch.clientX - self.touch[0]
        dy = touch.clientY - self.touch[1]
        if math.hypot(dx, dy) < 24:
            return
        self.touch_fired = True
        if abs(dx) > abs(dy):
            self.game.queue_dir(RIGHT if dx > 0 else LEFT)
        else:
            self.game.queue_dir(DOWN if dy > 0 else UP)

    def on_touch_end(self, event) -> None:
        if not self.touch_fired:
            self.restart_if_dead()
        self.touch = None

    # -- frame loop --------------------------------------------------------

    def frame(self, now: float) -> None:
        window.requestAnimationFrame(self.frame_proxy)
        dt = min(now - self.last, 100.0)
        self.last = now
        if not self.running:
            return

        if not self.dead and not self.paused:
            self.acc += dt
            if self.acc >= TICK:
                self.acc = 0.0
                self.advance()

        # Tweens run every frame regardless of the game clock.
        self.food_pop = min(1.0, self.food_pop + dt / 200)
        self.head_pulse += (1 - self.head_pulse) * min(1.0, dt / 90)
        for particle in self.particles:
            particle["life"] -= dt
            particle["x"] += particle["vx"] * dt * 0.001
            particle["y"] += particle["vy"] * dt * 0.001
        self.particles = [p for p in self.particles if p["life"] > 0]

        if self.badge_until and now > self.badge_until:
            self.badge_until = 0.0
            self.badge_el.classList.remove("show")

        self.draw()

    def advance(self) -> None:
        if self.mode == "demo":
            self.decision = decide(self.game)
            self.game.force_dir(self.decision.direction)
            if self.show_vision:
                self.show_tier(self.decision.tier)

        ate, died, won = self.game.step()
        self.score_el.textContent = f"Score: {self.game.score}"

        if ate:
            self.pulse(self.score_el)
            self.head_pulse = 1.15
            self.spawn_particles()
        if self.game.food is not None and self.game.food != self.last_food:
            self.last_food = self.game.food
            self.food_pop = 0.0

        if died or won:
            self.dead = True
            if self.game.score > self.best:
                self.best = self.game.score
                localStorage.setItem(BEST_KEY, str(self.best))
            self.show_best()
            self.game_over_el.textContent = (
                "You Win!\nTap to retry" if won else "Game Over\nTap to retry"
            )
            self.game_over_el.hidden = False
            self.restart_animation(self.game_over_el)
            self.badge_el.classList.remove("show")
            if died and not won:
                self.canvas.classList.remove("shake")
                self.canvas.offsetWidth  # noqa: B018 - forces a reflow
                self.canvas.classList.add("shake")

    def pulse(self, element) -> None:
        element.classList.remove("pop")
        element.offsetWidth  # noqa: B018 - forces a reflow
        element.classList.add("pop")

    def restart_animation(self, element) -> None:
        element.style.animation = "none"
        element.offsetWidth  # noqa: B018 - forces a reflow
        element.style.animation = ""

    def show_tier(self, tier: int) -> None:
        if tier == self.last_tier:
            return
        self.last_tier = tier
        label, color = TIER_STYLE[tier]
        self.badge_el.textContent = label
        self.badge_el.style.color = color
        self.badge_el.classList.add("show")
        self.badge_until = self.last + BADGE_MS

    def spawn_particles(self) -> None:
        hx, hy = self.game.snake[0]
        cx = hx * CELL + CELL / 2
        cy = hy * CELL + CELL / 2
        for _ in range(20):
            angle = random.random() * 2 * math.pi
            speed = 40 + random.random() * 100
            self.particles.append({
                "x": cx, "y": cy,
                "vx": math.cos(angle) * speed,
                "vy": math.sin(angle) * speed,
                "life": 300 + random.random() * 300,
                "max": 600,
            })

    # -- rendering ---------------------------------------------------------

    def draw(self) -> None:
        ctx = self.ctx
        game = self.game
        width, height = game.cols * CELL, game.rows * CELL
        alpha = 0.4 if self.dead else 1.0

        ctx.fillStyle = BACKGROUND
        ctx.fillRect(0, 0, width, height)

        ctx.strokeStyle = GRID
        ctx.lineWidth = 1
        ctx.beginPath()
        for x in range(game.cols + 1):
            ctx.moveTo(x * CELL + 0.5, 0)
            ctx.lineTo(x * CELL + 0.5, height)
        for y in range(game.rows + 1):
            ctx.moveTo(0, y * CELL + 0.5)
            ctx.lineTo(width, y * CELL + 0.5)
        ctx.stroke()

        if self.mode == "demo" and self.show_vision:
            self.draw_vision()

        ctx.globalAlpha = alpha

        ctx.fillStyle = BODY_COLOR
        for x, y in game.snake[1:]:
            ctx.fillRect(x * CELL + 1, y * CELL + 1, CELL - 2, CELL - 2)

        if game.food is not None:
            ease = 1 - (1 - self.food_pop) ** 3
            fx, fy = game.food
            ctx.fillStyle = FOOD_COLOR
            ctx.beginPath()
            ctx.arc(fx * CELL + CELL / 2, fy * CELL + CELL / 2,
                    max(0.5, (CELL / 2 - 2) * ease), 0, 2 * math.pi)
            ctx.fill()

        hx, hy = game.snake[0]
        cx, cy = hx * CELL + CELL / 2, hy * CELL + CELL / 2
        size = (CELL - 1) * self.head_pulse

        if not self.dead:
            ctx.globalAlpha = alpha * 0.18
            ctx.fillStyle = HEAD_COLOR
            ctx.fillRect(cx - (CELL + 6) / 2, cy - (CELL + 6) / 2, CELL + 6, CELL + 6)
            ctx.globalAlpha = alpha

        ctx.fillStyle = HEAD_COLOR
        ctx.fillRect(cx - size / 2, cy - size / 2, size, size)

        if not self.dead:
            off = CELL * 0.22
            eyes = {
                UP: ((-off, -off), (off, -off)),
                DOWN: ((-off, off), (off, off)),
                LEFT: ((-off, -off), (-off, off)),
                RIGHT: ((off, -off), (off, off)),
            }[game.direction]
            ctx.fillStyle = EYE_COLOR
            for ex, ey in eyes:
                ctx.beginPath()
                ctx.arc(cx + ex, cy + ey, max(1.5, CELL * 0.12), 0, 2 * math.pi)
                ctx.fill()

        ctx.globalAlpha = 1

        for particle in self.particles:
            t = max(0.0, particle["life"] / particle["max"])
            ctx.globalAlpha = t
            ctx.fillStyle = FOOD_COLOR
            ctx.beginPath()
            ctx.arc(particle["x"], particle["y"], 4 * t, 0, 2 * math.pi)
            ctx.fill()
        ctx.globalAlpha = 1

        if self.paused and not self.dead:
            ctx.fillStyle = "rgba(26, 26, 46, 0.7)"
            ctx.fillRect(0, 0, width, height)
            ctx.fillStyle = HEAD_COLOR
            ctx.font = "bold 22px ui-monospace, monospace"
            ctx.textAlign = "center"
            ctx.fillText("PAUSED", width / 2, height / 2)

    def draw_vision(self) -> None:
        """Show the AI's reasoning: reachable region shaded, path dashed."""
        ctx = self.ctx

        ctx.fillStyle = "rgba(108, 92, 231, 0.15)"
        for x, y in self.decision.reachable:
            ctx.fillRect(x * CELL + 1, y * CELL + 1, CELL - 2, CELL - 2)

        path = self.decision.path
        if len(path) < 2:
            return
        ctx.strokeStyle = "rgba(255, 118, 117, 0.9)"
        ctx.lineWidth = 2
        ctx.beginPath()
        for (ax, ay), (bx, by) in zip(path, path[1:]):
            x1, y1 = ax * CELL + CELL / 2, ay * CELL + CELL / 2
            x2, y2 = bx * CELL + CELL / 2, by * CELL + CELL / 2
            dx, dy = x2 - x1, y2 - y1
            steps = int(math.hypot(dx, dy) / 6)
            if steps < 1:
                ctx.moveTo(x1, y1)
                ctx.lineTo(x2, y2)
                continue
            for s in range(0, steps, 2):
                ctx.moveTo(x1 + s / steps * dx, y1 + s / steps * dy)
                ctx.lineTo(x1 + (s + 1) / steps * dx, y1 + (s + 1) / steps * dy)
        ctx.stroke()


# Pyodide runs this file as __main__; the guard lets test_ai.py import the
# rules and the AI without touching the DOM.
if __name__ == "__main__":
    UI()
