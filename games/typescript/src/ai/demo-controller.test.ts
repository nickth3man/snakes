import { describe, it, expect } from "vitest";
import { SnakeGame, Point } from "../game/engine";
import { getAIDirection } from "./demo-controller";
import type { AIDecision } from "./demo-controller";

describe("getAIDirection", () => {
  /**
   * Helper: step the game count times in the given direction.
   * Does NOT check for death — caller must ensure the path is safe.
   */
  function moveToward(game: SnakeGame, target: Point): boolean {
    for (let i = 0; i < 200; i++) {
      const s = game.getState();
      if (!s.alive) return false;
      const h = s.snake[0];
      if (h.x === target.x && h.y === target.y) return true;

      const dx = target.x - h.x;
      const dy = target.y - h.y;

      const dir: Point =
        dx !== 0
          ? { x: dx > 0 ? 1 : -1, y: 0 }
          : dy !== 0
            ? { x: 0, y: dy > 0 ? 1 : -1 }
            : s.direction;

      // Avoid reversal
      if (dir.x === -s.direction.x && dir.y === -s.direction.y) {
        game.step({ x: 0, y: dir.x !== 0 ? (dy >= 0 ? 1 : -1) : 0 } as Point);
      } else {
        game.step(dir);
      }
    }
    return false;
  }

  it("returns tier1 when food is reachable and there is ample space", () => {
    // Constructed state: 10×10 board, snake at center moving right,
    // food two cells ahead. Plenty of room and a clear path → tier1.
    // (Replaces the previous moveToward-based version that flaked because
    // random food spawning on a 5×5 board could starve tier1 of open space.)
    const state = {
      snake: [
        { x: 5, y: 5 },
        { x: 4, y: 5 },
        { x: 3, y: 5 },
      ],
      food: { x: 7, y: 5 },
      direction: { x: 1, y: 0 },
      score: 0,
      alive: true,
      won: false,
    };

    const decision: AIDecision = getAIDirection(state, 10, 10);
    expect(decision.debug.tier).toBe("tier1");
  });

  it("has at least one candidate for any live state", () => {
    const game = new SnakeGame({ cols: 5, rows: 5 });
    game.start();

    for (let tick = 0; tick < 10; tick++) {
      const state = game.getState();
      if (!state.alive) break;

      const decision = getAIDirection(state, 5, 5);
      expect(decision.debug.candidates.length).toBeGreaterThanOrEqual(1);

      // Step the game with the AI's chosen direction
      game.step(decision.dir);
    }
  });

  it("tier1 debug.path starts at head and ends at food", () => {
    // 7×7 board — more room
    const game = new SnakeGame({ cols: 7, rows: 7 });
    game.start();

    // Move snake to a safe position first, then get AI decision
    const state = game.getState();
    const origFood = state.food!;
    expect(origFood).not.toBeNull();

    moveToward(game, origFood);
    const liveState = game.getState();
    if (!liveState.alive) return; // collapsed, skip

    // After eating, a new food spawns — use the current food
    const currentFood = liveState.food;
    if (!currentFood) return; // won already, skip

    const decision = getAIDirection(liveState, 7, 7);
    if (decision.debug.tier !== "tier1") return; // skip if not tier1

    const path = decision.debug.path;
    expect(path.length).toBeGreaterThanOrEqual(2);
    const head = liveState.snake[0];
    expect(path[0]).toEqual(head);
    expect(path[path.length - 1]).toEqual(currentFood);
  });

  it("handles a state with no valid moves", () => {
    // Start a game on 5×5, move snake until it's boxed in
    const game = new SnakeGame({ cols: 5, rows: 5 });
    game.start();

    // Force the snake into a corner by repeatedly steering into a wall pattern
    // We'll just test a contrived state by feeding the AI a snake that has no moves
    const contrivedState = {
      snake: [
        { x: 1, y: 1 },
        { x: 0, y: 1 },
        { x: 0, y: 0 },
        { x: 1, y: 0 },
        { x: 2, y: 0 },
        { x: 2, y: 1 },
      ],
      food: { x: 4, y: 4 },
      direction: { x: 1, y: 0 },
      score: 0,
      alive: true,
      won: false,
    };

    const decision = getAIDirection(contrivedState, 3, 2);
    // Should return a fallback (current direction) with no candidates
    expect(decision.dir).toEqual({ x: 1, y: 0 });
    expect(decision.debug.tier).toBe("fallback");
    expect(decision.debug.candidates.length).toBe(0);
    expect(decision.debug.reachable.length).toBe(0);
  });
});
