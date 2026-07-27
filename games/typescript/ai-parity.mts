// Throwaway harness: drives the Rust wasm demo AI and the TypeScript demo
// controller over identical board states and asserts they agree.
//   npx tsx ai-parity.mts <path-to.wasm>
import { readFileSync } from "node:fs";
import { getAIDirection } from "./src/ai/demo-controller.js";
import type { Point } from "./src/game/engine.js";

const DIRS: Point[] = [
  { x: 0, y: -1 },
  { x: 1, y: 0 },
  { x: 0, y: 1 },
  { x: -1, y: 0 },
];
const TIERS = ["tier1", "tier2", "fallback"] as const;

const wasmPath = process.argv[2];
const { instance } = await WebAssembly.instantiate(readFileSync(wasmPath), {});
const m = instance.exports as any;

let checked = 0;
let mismatches = 0;
const report = (msg: string) => {
  if (mismatches < 6) console.log("  MISMATCH " + msg);
  mismatches++;
};

for (const [cols, rows] of [[30, 22], [18, 38]]) {
  for (let game = 0; game < 4; game++) {
    m.init(cols, rows, 1000 + game * 7919);
    let steps = 0;

    while (m.alive() && steps < 6000) {
      // Snapshot the wasm board in the shape the TypeScript AI expects.
      const len = m.length();
      const snakeCells = new Uint16Array(m.memory.buffer, m.snake_ptr(), len);
      const snake: Point[] = [...snakeCells].map((c) => ({
        x: c % cols,
        y: Math.floor(c / cols),
      }));
      const foodCell = m.food_cell();
      const food: Point | null =
        foodCell < 0 ? null : { x: foodCell % cols, y: Math.floor(foodCell / cols) };
      const direction = DIRS[m.direction()];
      const state = { snake, food, direction, score: m.score(), alive: true, won: false };

      const expected = getAIDirection(state, cols, rows);

      const tierIdx = m.ai_decide();
      const gotTier = TIERS[tierIdx];
      const gotReachable = m.ai_reachable_len();
      const gotPath = m.ai_path_len();
      m.step();
      const gotDir = DIRS[m.direction()];

      const where = `${cols}x${rows} game ${game} step ${steps} len ${len}`;
      if (gotTier !== expected.debug.tier) {
        report(`${where}: tier ${gotTier} vs ${expected.debug.tier}`);
      } else {
        if (gotDir.x !== expected.dir.x || gotDir.y !== expected.dir.y) {
          report(
            `${where}: dir (${gotDir.x},${gotDir.y}) vs (${expected.dir.x},${expected.dir.y}) [${gotTier}]`,
          );
        }
        if (gotReachable !== expected.debug.reachable.length) {
          report(
            `${where}: reachable ${gotReachable} vs ${expected.debug.reachable.length}`,
          );
        }
        if (gotPath !== expected.debug.path.length) {
          report(`${where}: path ${gotPath} vs ${expected.debug.path.length}`);
        }
      }

      checked++;
      steps++;
    }

    console.log(
      `${cols}x${rows} game ${game}: ${steps} steps, score ${m.score()}, ` +
        `length ${m.length()}${m.won() ? ", WON" : ""}`,
    );
  }
}

console.log(`\nchecked ${checked} decisions, ${mismatches} mismatches`);
process.exit(mismatches === 0 ? 0 : 1);
