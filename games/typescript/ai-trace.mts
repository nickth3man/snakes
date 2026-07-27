// Throwaway generator: records demo-AI decisions from the Rust module (which
// is checked against src/ai/demo-controller.ts by ai-parity.mts) so the Go,
// Python and Elm ports can be held to the same answers.
//   npx tsx ai-trace.mts <path-to.wasm> <out.json>
import { readFileSync, writeFileSync } from "node:fs";
import { getAIDirection } from "./src/ai/demo-controller.js";
import type { Point } from "./src/game/engine.js";

const DIRS: Point[] = [
  { x: 0, y: -1 },
  { x: 1, y: 0 },
  { x: 0, y: 1 },
  { x: -1, y: 0 },
];

const { instance } = await WebAssembly.instantiate(readFileSync(process.argv[2]), {});
const m = instance.exports as any;

type Sample = {
  cols: number;
  rows: number;
  /** Cell indices, head first. */
  snake: number[];
  food: number | null;
  dir: number;
  tier: number;
  chosen: number;
  reachable: number;
  path: number;
};

const samples: Sample[] = [];

for (const [cols, rows] of [[30, 22], [18, 38]] as const) {
  for (let game = 0; game < 3; game++) {
    m.init(cols, rows, 4242 + game * 977);
    let steps = 0;
    while (m.alive() && steps < 5000) {
      const len = m.length();
      const cells = new Uint16Array(m.memory.buffer, m.snake_ptr(), len);
      const snake = [...cells];
      const foodCell = m.food_cell();
      const food: number | null = foodCell < 0 ? null : foodCell;
      const dir = m.direction();

      // Cross-check against the TypeScript controller as we go.
      const expected = getAIDirection(
        {
          snake: snake.map((c) => ({ x: c % cols, y: Math.floor(c / cols) })),
          food: food === null ? null : { x: food % cols, y: Math.floor(food / cols) },
          direction: DIRS[dir],
          score: m.score(),
          alive: true,
          won: false,
        },
        cols,
        rows,
      );

      const tier = m.ai_decide();
      const reachable = m.ai_reachable_len();
      const path = m.ai_path_len();
      m.step();
      const chosen = m.direction();

      const tierName = ["tier1", "tier2", "fallback"][tier];
      if (tierName !== expected.debug.tier) {
        throw new Error(`tier drift at ${cols}x${rows} step ${steps}`);
      }

      // Keep a thin slice: enough coverage of all three tiers, small file.
      if (steps % 97 === 0) {
        samples.push({ cols, rows, snake, food, dir, tier, chosen, reachable, path });
      }
      steps++;
    }
  }
}

writeFileSync(process.argv[3], JSON.stringify(samples));
console.log(`wrote ${samples.length} samples to ${process.argv[3]}`);
const byTier = [0, 1, 2].map((t) => samples.filter((s) => s.tier === t).length);
console.log(`tier1 ${byTier[0]} · tier2 ${byTier[1]} · fallback ${byTier[2]}`);
