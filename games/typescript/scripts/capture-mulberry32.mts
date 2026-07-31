// Captures the first 10 outputs of mulberry32(42) from the real
// TypeScript implementation, so the Go port can verify byte-for-byte.

import { mulberry32 } from "../src/benchmark/rng";

const r = mulberry32(42);
const out: number[] = [];
for (let i = 0; i < 10; i++) {
  out.push(r());
}
console.log(JSON.stringify(out));
