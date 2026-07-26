import { describe, it, expect } from "vitest";
import { SnakeGame, Point } from "./engine";

function isOpposite(a: Point, b: Point): boolean {
  return a.x === -b.x && a.y === -b.y;
}

/**
 * Navigate the snake to a target cell.
 * Moves horizontally then vertically, handling the reversal guard.
 * Returns true if the target was reached, false if the snake died.
 */
function navigateTo(game: SnakeGame, target: Point): boolean {
  for (let i = 0; i < 1000; i++) {
    const s = game.getState();
    if (!s.alive) return false;
    const h = s.snake[0];
    if (h.x === target.x && h.y === target.y) return true;

    const dx = target.x - h.x;
    const dy = target.y - h.y;

    // Try horizontal
    if (dx !== 0) {
      const dir: Point = { x: dx > 0 ? 1 : -1, y: 0 };
      if (!isOpposite(dir, s.direction)) {
        game.step(dir);
        continue;
      }
    }

    // Try vertical
    if (dy !== 0) {
      const dir: Point = { x: 0, y: dy > 0 ? 1 : -1 };
      if (!isOpposite(dir, s.direction)) {
        game.step(dir);
        continue;
      }
    }

    // Blocked — detour perpendicular
    if (s.direction.x !== 0) {
      game.step({ x: 0, y: dy >= 0 ? 1 : -1 });
    } else {
      game.step({ x: dx >= 0 ? 1 : -1, y: 0 });
    }
  }
  return false;
}

describe("SnakeGame", () => {
  it("initial state", () => {
    const game = new SnakeGame({ cols: 30, rows: 22 });
    game.start();

    const s = game.getState();
    expect(s.alive).toBe(true);
    expect(s.won).toBe(false);
    expect(s.score).toBe(0);
    expect(s.direction).toEqual({ x: 1, y: 0 });
    expect(s.snake).toHaveLength(3);
    // midRow = 11
    expect(s.snake[0]).toEqual({ x: 5, y: 11 });
    expect(s.snake[1]).toEqual({ x: 4, y: 11 });
    expect(s.snake[2]).toEqual({ x: 3, y: 11 });
    expect(s.food).not.toBeNull();
  });

  it("move right", () => {
    const game = new SnakeGame({ cols: 30, rows: 22 });
    game.start();

    const result = game.step({ x: 1, y: 0 });
    expect(result.died).toBe(false);
    expect(result.won).toBe(false);
    expect(result.ate).toBe(false);

    const s = game.getState();
    expect(s.alive).toBe(true);
    // Head advances, tail pops: [(6,11), (5,11), (4,11)]
    expect(s.snake).toHaveLength(3);
    expect(s.snake[0]).toEqual({ x: 6, y: 11 });
    expect(s.snake[1]).toEqual({ x: 5, y: 11 });
    expect(s.snake[2]).toEqual({ x: 4, y: 11 });
  });

  it("reversal rejection", () => {
    const game = new SnakeGame({ cols: 30, rows: 22 });
    game.start();

    // Try to go left (opposite of starting right)
    const result = game.step({ x: -1, y: 0 });
    expect(result.died).toBe(false);
    expect(result.won).toBe(false);

    const s = game.getState();
    // Snake should continue right
    expect(s.direction).toEqual({ x: 1, y: 0 });
    expect(s.snake[0]).toEqual({ x: 6, y: 11 });
  });

  it("wall death", () => {
    const game = new SnakeGame({ cols: 30, rows: 22 });
    game.start();

    // Head starts at (5,11) going right.
    // Turn up first to avoid reversal when going left.
    game.step({ x: 0, y: -1 }); // head → (5,10) going up
    // Now go left 6 times: 5→4→3→2→1→0→-1 (out of bounds)
    game.step({ x: -1, y: 0 }); // →4
    game.step({ x: -1, y: 0 }); // →3
    game.step({ x: -1, y: 0 }); // →2
    game.step({ x: -1, y: 0 }); // →1
    game.step({ x: -1, y: 0 }); // →0
    const result = game.step({ x: -1, y: 0 }); // →-1 out of bounds
    expect(result.died).toBe(true);
    expect(result.won).toBe(false);

    const s = game.getState();
    expect(s.alive).toBe(false);
  });

  it("self death", () => {
    const game = new SnakeGame({ cols: 30, rows: 22 });
    game.start();

    // First, eat food to grow the snake to 4 cells
    const initialState = game.getState();
    const food = initialState.food!;
    const reached = navigateTo(game, food);
    expect(reached).toBe(true);

    let state = game.getState();
    expect(state.score).toBe(1);
    expect(state.snake.length).toBe(4);
    expect(state.alive).toBe(true);

    // Now do a tight loop to cause self-collision
    const dir = state.direction;

    // Determine the collision sequence based on current direction
    let turn1: Point;
    let turn2: Point;
    let turn3: Point;

    if (dir.x === 1) {
      // going right: up, left, down → collision
      turn1 = { x: 0, y: -1 };
      turn2 = { x: -1, y: 0 };
      turn3 = { x: 0, y: 1 };
    } else if (dir.x === -1) {
      // going left: up, right, down → collision
      turn1 = { x: 0, y: -1 };
      turn2 = { x: 1, y: 0 };
      turn3 = { x: 0, y: 1 };
    } else if (dir.y === -1) {
      // going up: right, down, left → collision
      turn1 = { x: 1, y: 0 };
      turn2 = { x: 0, y: 1 };
      turn3 = { x: -1, y: 0 };
    } else {
      // going down: left, up, right → collision
      turn1 = { x: -1, y: 0 };
      turn2 = { x: 0, y: -1 };
      turn3 = { x: 1, y: 0 };
    }

    game.step(turn1);
    state = game.getState();
    expect(state.alive).toBe(true);

    game.step(turn2);
    state = game.getState();
    expect(state.alive).toBe(true);

    const result = game.step(turn3);
    expect(result.died).toBe(true);
    expect(game.getState().alive).toBe(false);
  });

  it("eat food", () => {
    const game = new SnakeGame({ cols: 30, rows: 22 });
    game.start();

    const initialState = game.getState();
    const food = initialState.food!;
    const reached = navigateTo(game, food);
    expect(reached).toBe(true);

    const state = game.getState();
    expect(state.score).toBe(1);
    // Snake should have grown by 1 (no tail pop)
    expect(state.snake.length).toBe(4);
    // A new food should have spawned
    expect(state.food).not.toBeNull();
    // The new food should be different from the eaten one
    expect(state.food).not.toEqual(food);
  });

  it("food never on snake", () => {
    const game = new SnakeGame({ cols: 30, rows: 22 });
    game.start();

    // Eat food once
    const initialFood = game.getState().food!;
    navigateTo(game, initialFood);

    const state = game.getState();
    const newFood = state.food!;
    const onSnake = state.snake.some(
      (s) => s.x === newFood.x && s.y === newFood.y,
    );
    expect(onSnake).toBe(false);
  });

  it("full-board win", () => {
    // 3×1 board: snake starts filling all 3 cells → immediate win
    const game = new SnakeGame({ cols: 3, rows: 1 });
    game.start();

    const s = game.getState();
    expect(s.won).toBe(true);
    expect(s.alive).toBe(false);
    expect(s.food).toBeNull();
  });
});
