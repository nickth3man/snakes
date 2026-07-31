// Generates the golden fixture for the leaderboard port. Run from
// games/typescript/ with `npx tsx scripts/gen-leaderboard-golden.mts`
// and pipe the output to ../../testdata/leaderboard-golden.json.
//
// The Go leaderboard test compares its byte-for-byte output to
// this fixture so all six language ports share one source of
// truth for the leaderboard strings.

import { generateNpcEntries, mergeWithPlayer, formatRow } from "../src/leaderboard/leaderboard-core";

const npc = generateNpcEntries();

const cases = [
  { name: "no_player", player: null },
  { name: "low_best",  player: { name: "YOU" as const, score: 50 } },
  { name: "mid_best",  player: { name: "YOU" as const, score: 156 } },
  { name: "top_best",  player: { name: "YOU" as const, score: 500 } },
  { name: "tie_best",  player: { name: "YOU" as const, score: npc[9].score } },
];

const out: Record<string, unknown> = {
  version: 1,
  seed: 0x514d3a75,
  name_count: 68,
  score_max: 320,
  score_min: 8,
  curve_exponent: 0.55,
  jitter_amp: 7,
  name_display_max: 22,
  npc_scores: npc.map((e) => ({ rank: e.rank, score: e.score })),
  cases: {} as Record<string, string[]>,
};

for (const c of cases) {
  const rows = mergeWithPlayer(npc, c.player);
  (out.cases as Record<string, string[]>)[c.name] = rows.map(formatRow);
}

process.stdout.write(JSON.stringify(out, null, 2) + "\n");
