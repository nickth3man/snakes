import { mulberry32 } from "../benchmark/rng";
import { NAMES, NAME_COUNT } from "./names";

/* ──────────── Types ──────────── */

export interface NpcEntry {
  /** 1..68, the curated power rank (NOT the sorted score rank). */
  rank: number;
  /** Full display name, never truncated. */
  name: string;
  score: number;
}

export interface PlayerEntry {
  name: "YOU";
  score: number;
}

export type LeaderboardRow =
  | {
      sortRank: number;
      score: number;
      isPlayer: false;
      name: string;
      powerRank: number;
    }
  | {
      sortRank: number;
      score: number;
      isPlayer: true;
      name: "YOU";
      powerRank: null;
    };

/* ──────────── Score curve ────────────
 * rank 1 (most powerful) → ~320, rank 68 (least) → ~8.
 * Power-law exponent < 1 gives wide gaps at the top, tight at the bottom.
 * ±7 jitter yields occasional 2-3 position inversions — organic feel.
 * Calibrated to this game: 660-cell board, headless AI max ~213.
 */
export const SCORE_MAX = 320;
export const SCORE_MIN = 8;
export const CURVE_EXPONENT = 0.55;
export const JITTER_AMP = 7;
/** Fixed seed → identical leaderboard on every device / reload. */
export const SEED = 0x514d3a75;

export function scoreCurveBase(rank1: number): number {
  const t = (rank1 - 1) / (NAME_COUNT - 1);
  const base =
    SCORE_MAX - (SCORE_MAX - SCORE_MIN) * Math.pow(t, CURVE_EXPONENT);
  return Math.round(base);
}

/**
 * Deterministic generation: one RNG draw per figure, consumed in rank
 * order, so the sequence is reproducible from the fixed seed.
 */
export function generateNpcEntries(seed: number = SEED): NpcEntry[] {
  const rng = mulberry32(seed >>> 0);
  const out: NpcEntry[] = [];
  for (let i = 0; i < NAME_COUNT; i++) {
    const rank = i + 1;
    const base = scoreCurveBase(rank);
    const jitter = Math.round((rng() * 2 - 1) * JITTER_AMP);
    const score = Math.max(1, base + jitter);
    out.push({ rank, name: NAMES[i], score });
  }
  return out;
}

/* ──────────── Merge with player (pure) ──────────── */

export function mergeWithPlayer(
  npc: NpcEntry[],
  player: PlayerEntry | null,
): LeaderboardRow[] {
  const rows: LeaderboardRow[] = npc.map((e) => ({
    sortRank: 0,
    score: e.score,
    isPlayer: false as const,
    name: e.name,
    powerRank: e.rank,
  }));
  if (player && player.score > 0) {
    rows.push({
      sortRank: 0,
      score: player.score,
      isPlayer: true as const,
      name: "YOU",
      powerRank: null,
    });
  }
  // Desc by score; ties → player first.
  rows.sort(
    (a, b) =>
      b.score - a.score ||
      (a.isPlayer ? -1 : 0) - (b.isPlayer ? -1 : 0),
  );
  rows.forEach((r, i) => {
    r.sortRank = i + 1;
  });
  return rows;
}

/* ──────────── Display formatting (pure) ──────────── */

export const NAME_DISPLAY_MAX = 22;

export function truncateName(
  name: string,
  max: number = NAME_DISPLAY_MAX,
): string {
  return name.length > max ? name.slice(0, max - 1) + "\u2026" : name;
}

/**
 * Monospace row layout:
 *   "▶  7. YOU                      156 (YOU)"
 *   "   10. Michael Jordan          269"
 */
export function formatRow(row: LeaderboardRow): string {
  const tag = row.isPlayer ? "\u25B6 " : "  ";
  const rank = String(row.sortRank).padStart(2, " ");
  const name = truncateName(row.name).padEnd(NAME_DISPLAY_MAX + 2, " ");
  const score = String(row.score).padStart(4, " ");
  const suffix = row.isPlayer ? " (YOU)" : "";
  return `${tag}${rank}. ${name}${score}${suffix}`;
}
