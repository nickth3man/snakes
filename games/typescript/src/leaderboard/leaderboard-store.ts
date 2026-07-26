import {
  generateNpcEntries,
  mergeWithPlayer,
  LeaderboardRow,
  NpcEntry,
  SEED,
} from "./leaderboard-core";

/**
 * localStorage IO boundary for the leaderboard.
 *
 * NPC entries are generated once from the fixed seed and persisted; the
 * player row is merged live at every read (from the GameScene-owned
 * "snakey-best-score" key), so it is never stale and never rewritten.
 * Safari private mode: setItem throws → caught → in-memory memo only.
 */

const KEY = "snakey-leaderboard";
const BEST_KEY = "snakey-best-score"; // READ-ONLY — owned by GameScene
const DOC_VERSION = 1;

interface LeaderboardDoc {
  version: number;
  seed: number;
  generatedAt: number;
  entries: NpcEntry[]; // 68 NPC entries only — player merged at read time
}

// Module-level memo so localStorage is hit at most once per session.
let memo: LeaderboardDoc | null = null;

function readBestScore(): number {
  try {
    const raw = localStorage.getItem(BEST_KEY);
    if (raw === null) return 0;
    const n = parseInt(raw, 10);
    return Number.isFinite(n) && n > 0 ? n : 0;
  } catch {
    return 0; // localStorage unavailable
  }
}

function loadDoc(): LeaderboardDoc {
  if (memo) return memo;

  let doc: LeaderboardDoc | null = null;
  try {
    const raw = localStorage.getItem(KEY);
    if (raw) doc = JSON.parse(raw) as LeaderboardDoc;
  } catch {
    // parse error or unavailable
  }

  if (
    !doc ||
    doc.version !== DOC_VERSION ||
    doc.seed !== SEED ||
    !Array.isArray(doc.entries) ||
    doc.entries.length === 0
  ) {
    doc = {
      version: DOC_VERSION,
      seed: SEED,
      generatedAt: Date.now(),
      entries: generateNpcEntries(SEED),
    };
    try {
      localStorage.setItem(KEY, JSON.stringify(doc));
    } catch {
      // Safari private mode — keep in-memory memo only
    }
  }
  memo = doc;
  return doc;
}

/** Public entry point called by the UI. Cheap; safe to call per session. */
export function getLeaderboard(): LeaderboardRow[] {
  const doc = loadDoc();
  const best = readBestScore();
  const player = best > 0 ? { name: "YOU" as const, score: best } : null;
  return mergeWithPlayer(doc.entries, player);
}

/** For tests only. */
export function __resetMemo(): void {
  memo = null;
}
