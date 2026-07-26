"""Snake in Python, played in the browser.

Pyodide is CPython compiled to WebAssembly, so this is the real interpreter --
dataclasses, ``random.randrange`` and all -- reaching out to the DOM through the
``js`` bridge. Nothing here is transpiled to JavaScript.
"""

from __future__ import annotations

import random
from dataclasses import dataclass, field

from js import document, localStorage, window
from pyodide.ffi import create_proxy

COLS = 24
ROWS = 24
CELL = 24
BASE_TICK = 120.0  # ms between moves at length 3
MIN_TICK = 65.0  # floor, reached after ~28 apples
BEST_KEY = "snake-python-best"

BACKGROUND = "#1a1a2e"
GRID = "#16213e"
HEAD = "#00cec9"
BODY = "#6c5ce7"
FOOD = "#ff7675"
EYE = "#0a0a1e"

UP = (0, -1)
RIGHT = (1, 0)
DOWN = (0, 1)
LEFT = (-1, 0)

KEY_DIRS = {
    "ArrowUp": UP,
    "KeyW": UP,
    "ArrowRight": RIGHT,
    "KeyD": RIGHT,
    "ArrowDown": DOWN,
    "KeyS": DOWN,
    "ArrowLeft": LEFT,
    "KeyA": LEFT,
}


@dataclass
class Game:
    """Board state and rules. No DOM, no drawing."""

    snake: list[tuple[int, int]] = field(default_factory=list)
    food: tuple[int, int] = (0, 0)
    direction: tuple[int, int] = RIGHT
    pending: tuple[int, int] = RIGHT
    score: int = 0
    alive: bool = False
    won: bool = False

    def reset(self) -> None:
        row = ROWS // 2
        self.snake = [(4, row), (3, row), (2, row)]
        self.direction = self.pending = RIGHT
        self.score = 0
        self.alive = True
        self.won = False
        self.spawn_food()

    def spawn_food(self) -> None:
        """Pick uniformly among the cells the snake does not occupy."""
        taken = set(self.snake)
        free = [
            (x, y) for y in range(ROWS) for x in range(COLS) if (x, y) not in taken
        ]
        if free:
            self.food = random.choice(free)

    def turn(self, direction: tuple[int, int]) -> None:
        """Queue a direction change, ignoring reversals into the neck."""
        dx, dy = direction
        if len(self.snake) > 1 and (dx, dy) == (-self.direction[0], -self.direction[1]):
            return
        self.pending = direction

    def step(self) -> bool:
        """Advance one tick. Returns True if the snake ate this tick."""
        if not self.alive:
            return False

        self.direction = self.pending
        (hx, hy), (dx, dy) = self.snake[0], self.direction
        head = (hx + dx, hy + dy)

        if not (0 <= head[0] < COLS and 0 <= head[1] < ROWS):
            self.alive = False
            return False

        ate = head == self.food
        # Chasing your own tail is fine: it moves out of the way this tick.
        kept = self.snake if ate else self.snake[:-1]
        if head in kept:
            self.alive = False
            return False

        self.snake = [head, *kept]
        if ate:
            self.score += 10
            if len(self.snake) == COLS * ROWS:
                self.alive = False
                self.won = True
            else:
                self.spawn_food()
        return ate

    @property
    def tick_ms(self) -> float:
        """Milliseconds per move: quicker with every apple, down to a floor."""
        return max(MIN_TICK, BASE_TICK - 2 * (self.score // 10))


class UI:
    """Wires the game to the canvas, the keyboard and the score boxes."""

    def __init__(self) -> None:
        canvas = document.getElementById("board")
        canvas.width = COLS * CELL
        canvas.height = ROWS * CELL

        self.ctx = canvas.getContext("2d")
        self.stage = document.querySelector(".stage")
        self.overlay = document.getElementById("overlay")
        self.overlay_title = document.getElementById("overlay-title")
        self.overlay_text = document.getElementById("overlay-text")
        self.score_el = document.getElementById("score")
        self.best_el = document.getElementById("best")

        self.game = Game()
        self.game.reset()
        self.running = False
        self.paused = False
        self.acc = 0.0
        self.last = 0.0
        self.touch: tuple[float, float] | None = None

        self.best = int(localStorage.getItem(BEST_KEY) or 0)
        self.best_el.textContent = str(self.best)

        # Proxies must outlive the call that registers them, so keep references.
        self._proxies = [
            create_proxy(self.on_key),
            create_proxy(self.on_touch_start),
            create_proxy(self.on_touch_end),
            create_proxy(self.on_click),
            create_proxy(self.frame),
        ]
        window.addEventListener("keydown", self._proxies[0])
        self.stage.addEventListener("touchstart", self._proxies[1])
        self.stage.addEventListener("touchend", self._proxies[2])
        self.stage.addEventListener("click", self._proxies[3])

    # -- lifecycle ---------------------------------------------------------

    def start(self) -> None:
        self.draw()
        self.show_overlay("PYTHON SNAKE", "Press <kbd>SPACE</kbd> or tap to start")
        window.requestAnimationFrame(self._proxies[4])

    def new_game(self) -> None:
        self.game.reset()
        self.acc = 0.0
        self.paused = False
        self.running = True
        self.score_el.textContent = "0"
        self.overlay.hidden = True
        self.draw()

    def show_overlay(self, title: str, text: str) -> None:
        self.overlay_title.textContent = title
        self.overlay_text.innerHTML = text
        self.overlay.hidden = False

    def frame(self, now: float) -> None:
        window.requestAnimationFrame(self._proxies[4])
        if not self.running or self.paused:
            self.last = now
            return

        self.acc += now - self.last
        self.last = now
        if self.acc < self.game.tick_ms:
            return
        self.acc = 0.0

        self.game.step()
        self.score_el.textContent = str(self.game.score)
        if not self.game.alive:
            self.finish()
        self.draw()

    def finish(self) -> None:
        self.running = False
        if self.game.score > self.best:
            self.best = self.game.score
            localStorage.setItem(BEST_KEY, str(self.best))
            self.best_el.textContent = str(self.best)
        self.show_overlay(
            "PERFECT GAME" if self.game.won else "GAME OVER",
            f"Score <b>{self.game.score}</b> &middot; length {len(self.game.snake)}"
            "<br />Press <kbd>SPACE</kbd> or tap to play again",
        )

    # -- input -------------------------------------------------------------

    def on_key(self, event) -> None:
        code = event.code
        if code in KEY_DIRS:
            event.preventDefault()
            if self.running and not self.paused:
                self.game.turn(KEY_DIRS[code])
            elif not self.paused:
                self.new_game()
        elif code in ("Space", "Enter"):
            event.preventDefault()
            if not self.running:
                self.new_game()
        elif code == "KeyP" and self.running:
            self.paused = not self.paused
            if self.paused:
                self.show_overlay("PAUSED", "Press <kbd>P</kbd> to resume")
            else:
                self.overlay.hidden = True

    def on_touch_start(self, event) -> None:
        touch = event.touches.item(0)
        self.touch = (touch.clientX, touch.clientY)

    def on_touch_end(self, event) -> None:
        if self.touch is None:
            return
        touch = event.changedTouches.item(0)
        dx = touch.clientX - self.touch[0]
        dy = touch.clientY - self.touch[1]
        self.touch = None

        if not self.running:
            self.new_game()
        elif abs(dx) > 24 or abs(dy) > 24:
            if abs(dx) > abs(dy):
                self.game.turn(RIGHT if dx > 0 else LEFT)
            else:
                self.game.turn(DOWN if dy > 0 else UP)

    def on_click(self, event) -> None:
        if not self.running and not self.paused:
            self.new_game()

    # -- rendering ---------------------------------------------------------

    def draw(self) -> None:
        ctx = self.ctx
        width, height = COLS * CELL, ROWS * CELL

        ctx.fillStyle = BACKGROUND
        ctx.fillRect(0, 0, width, height)

        ctx.strokeStyle = GRID
        ctx.lineWidth = 1
        ctx.beginPath()
        for i in range(1, COLS):
            ctx.moveTo(i * CELL + 0.5, 0)
            ctx.lineTo(i * CELL + 0.5, height)
        for j in range(1, ROWS):
            ctx.moveTo(0, j * CELL + 0.5)
            ctx.lineTo(width, j * CELL + 0.5)
        ctx.stroke()

        fx, fy = self.game.food
        ctx.fillStyle = FOOD
        ctx.beginPath()
        ctx.arc(fx * CELL + CELL / 2, fy * CELL + CELL / 2, CELL * 0.32, 0, 6.2832)
        ctx.fill()

        ctx.fillStyle = BODY
        for x, y in self.game.snake[1:]:
            self.round_rect(x * CELL + 2, y * CELL + 2, CELL - 4, CELL - 4, 5)

        hx, hy = self.game.snake[0]
        px, py = hx * CELL, hy * CELL
        ctx.fillStyle = HEAD
        self.round_rect(px + 1, py + 1, CELL - 2, CELL - 2, 6)
        ctx.fillStyle = EYE
        ctx.beginPath()
        ctx.arc(px + CELL * 0.35, py + CELL * 0.38, CELL * 0.09, 0, 6.2832)
        ctx.arc(px + CELL * 0.65, py + CELL * 0.38, CELL * 0.09, 0, 6.2832)
        ctx.fill()

    def round_rect(self, x: float, y: float, w: float, h: float, r: float) -> None:
        self.ctx.beginPath()
        self.ctx.roundRect(x, y, w, h, r)
        self.ctx.fill()


UI().start()
