import { SnakeGame } from "../game/engine";
import { getAIDirection } from "../ai/demo-controller";
import { mulberry32 } from "./rng";
import { mean, median, percentile } from "./stats";
import type { BenchmarkResult } from "./types";
import { COLS, ROWS } from "../config";
import fs from "node:fs";
import path from "node:path";

interface Args {
  runs: number;
  seed: number;
  out: string;
  verbose: boolean;
}

function parseArgs(): Args {
  const args = process.argv.slice(2);
  const parsed: Args = { runs: 100, seed: 0, out: "public/benchmark.json", verbose: false };
  for (let i = 0; i < args.length; i++) {
    switch (args[i]) {
      case "--runs":
        parsed.runs = parseInt(args[++i], 10) || 100;
        break;
      case "--seed":
        parsed.seed = parseInt(args[++i], 10) || 0;
        break;
      case "--out":
        parsed.out = args[++i] || "public/benchmark.json";
        break;
      case "--verbose":
        parsed.verbose = true;
        break;
    }
  }
  if (parsed.seed === 0) parsed.seed = Date.now();
  return parsed;
}

function runOne(seed: number): { score: number; steps: number; won: boolean } {
  const rng = mulberry32(seed);
  const engine = new SnakeGame({ cols: COLS, rows: ROWS, rng });
  engine.start();
  let steps = 0;
  const MAX_STEPS = 10000;
  while (engine.getState().alive && steps < MAX_STEPS) {
    const state = engine.getState();
    const ai = getAIDirection(state, COLS, ROWS);
    engine.step(ai.dir);
    steps++;
    if (!engine.getState().alive) break;
  }
  const state = engine.getState();
  return { score: state.score, steps, won: state.won };
}

const args = parseArgs();
const scores: number[] = [];
let totalSteps = 0;
let wins = 0;

console.log(`Running ${args.runs} games (seed=${args.seed})...`);

for (let i = 0; i < args.runs; i++) {
  const result = runOne(args.seed + i);
  scores.push(result.score);
  totalSteps += result.steps;
  if (result.won) wins++;
  if (args.verbose) {
    console.log(`  Game ${i + 1}: score=${result.score} steps=${result.steps} won=${result.won}`);
  }
}

const maxScore = Math.max(...scores);
const minScore = Math.min(...scores);

const benchmark: BenchmarkResult = {
  runs: args.runs,
  seed: args.seed,
  timestamp: new Date().toISOString(),
  version: 1,
  mean: mean(scores),
  median: median(scores),
  max: maxScore,
  min: minScore,
  p90: percentile(scores, 90),
  winRate: wins / args.runs,
  avgSteps: Math.round(totalSteps / args.runs),
  scores: scores.slice(0, 200),
};

const outPath = path.resolve(args.out);
fs.mkdirSync(path.dirname(outPath), { recursive: true });
fs.writeFileSync(outPath, JSON.stringify(benchmark, null, 2));

console.log(`\nDone. Results written to ${args.out}:`);
console.log(`  Runs:   ${benchmark.runs}`);
console.log(`  Mean:   ${benchmark.mean.toFixed(1)}`);
console.log(`  Median: ${benchmark.median}`);
console.log(`  Max:    ${benchmark.max}`);
console.log(`  Min:    ${benchmark.min}`);
console.log(`  P90:    ${benchmark.p90}`);
console.log(`  Win %:  ${(benchmark.winRate * 100).toFixed(1)}%`);
console.log(`  Steps:  ${benchmark.avgSteps} avg`);
