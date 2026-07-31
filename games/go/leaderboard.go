package main

import (
	"math"
	"unicode/utf8"
)

// The leaderboard core: deterministic generation of 68 NPC entries
// from a fixed seed, merge-with-player logic, and the monospace row
// formatter. Mirrors games/typescript/src/leaderboard/leaderboard-core.ts
// byte-for-byte so the same golden fixture drives all six ports.
//
// No DOM, no syscall/js, no build tag — compiles under `go test`
// on the host just like game.go and ai.go.

const (
	ScoreMax        = 320
	ScoreMin        = 8
	CurveExponent   = 0.55
	JitterAmp       = 7
	LeaderboardSeed = uint32(0x514d3a75)
	NameCount       = 68
	NameDisplayMax  = 22
)

// NpcEntry is one row from the curated 68-name list.
type NpcEntry struct {
	Rank  int    // 1..68, curated power rank
	Name  string // never truncated
	Score int
}

// PlayerEntry is the live player best-score, merged at read time.
type PlayerEntry struct {
	Score int
}

// LeaderboardRow is the post-merge shape with a sort rank.
type LeaderboardRow struct {
	SortRank  int
	Score     int
	IsPlayer  bool
	Name      string
	PowerRank int // 0 for player
}

// ScoreCurveBase returns the deterministic base score for a rank.
// Rank 1 → ~ScoreMax, rank NameCount → ~ScoreMin.
func ScoreCurveBase(rank1 int) int {
	t := float64(rank1-1) / float64(NameCount-1)
	base := ScoreMax - (ScoreMax-ScoreMin)*math.Pow(t, CurveExponent)
	return int(math.Round(base))
}

// mulberry32 is the TypeScript implementation, ported line-for-line.
// Returns values in [0, 1) as a float64, identical to Math.imul
// path that the TypeScript version walks. Two calls with the same
// seed produce identical sequences.
//
// TS keeps `a` as a 32-bit signed integer (because of `a | 0` and
// the `>>>` / `^` / `+` ops all coerce via ToInt32). Go's `int`
// is platform-sized; we use `int32` throughout and rely on
// overflow rules to match JavaScript's int32 arithmetic.
func mulberry32(seed uint32) func() float64 {
	a := int32(seed)
	return func() float64 {
		// `a = a + 0x6d2b79f5 | 0` in TS becomes int32 wrap on add.
		a = a + int32(0x6d2b79f5)
		// `let t = Math.imul(a ^ (a >>> 15), 1 | a)`.
		// a >>> 15 is unsigned right shift on the int32 bit pattern.
		t := imul32(a^int32(uint32(a)>>15), 1|a)
		// `t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t`.
		t = (t + imul32(t^int32(uint32(t)>>7), 61|t)) ^ t
		// `((t ^ (t >>> 14)) >>> 0) / 4294967296`.
		return float64(uint32(t)^uint32(int32(uint32(t)>>14))) / 4294967296.0
	}
}

// imul32 emulates JavaScript's Math.imul: 32-bit signed
// multiplication. Go's int is platform-dependent; we cast
// to int32 first to force the wrap, then back.
func imul32(a, b int32) int32 {
	return a * b
}

// GenerateNpcEntries builds 68 NPC entries from the fixed seed.
// Same seed → same scores. Jitter ±JitterAmp, floored at 1.
func GenerateNpcEntries(seed uint32) []NpcEntry {
	rng := mulberry32(seed)
	out := make([]NpcEntry, 0, NameCount)
	for i := 0; i < NameCount; i++ {
		rank := i + 1
		base := ScoreCurveBase(rank)
		jitter := int(math.Round((rng()*2 - 1) * float64(JitterAmp)))
		score := base + jitter
		if score < 1 {
			score = 1
		}
		out = append(out, NpcEntry{Rank: rank, Name: NAMES[i], Score: score})
	}
	return out
}

// MergeWithPlayer takes the NPC list and an optional player entry,
// sorts descending by score (ties → player first), and numbers the
// rows 1..N.
func MergeWithPlayer(npc []NpcEntry, p *PlayerEntry) []LeaderboardRow {
	rows := make([]LeaderboardRow, 0, len(npc)+1)
	for _, e := range npc {
		rows = append(rows, LeaderboardRow{
			SortRank:  0,
			Score:     e.Score,
			IsPlayer:  false,
			Name:      e.Name,
			PowerRank: e.Rank,
		})
	}
	if p != nil && p.Score > 0 {
		rows = append(rows, LeaderboardRow{
			SortRank:  0,
			Score:     p.Score,
			IsPlayer:  true,
			Name:      "YOU",
			PowerRank: 0,
		})
	}
	// Desc by score; ties → player first.
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0; j-- {
			a, b := rows[j-1], rows[j]
			if b.Score > a.Score || (b.Score == a.Score && b.IsPlayer && !a.IsPlayer) {
				rows[j-1], rows[j] = b, a
			} else {
				break
			}
		}
	}
	for i := range rows {
		rows[i].SortRank = i + 1
	}
	return rows
}

// TruncateName slices to NameDisplayMax-1 chars and appends "…".
// Short names are unchanged.
// TruncateName slices to NameDisplayMax-1 runes and appends "…".
// Short names are unchanged. Length is rune-count to match
// JavaScript's String.prototype.length, which counts UTF-16
// code units (so an ellipsis "…" counts as 1, not 3 bytes).
func TruncateName(name string) string {
	if utf8.RuneCountInString(name) <= NameDisplayMax {
		return name
	}
	runes := []rune(name)
	return string(runes[:NameDisplayMax-1]) + "…"
}

// padRight returns s padded with spaces on the right to rune-width n.
// Matches the TypeScript padEnd behavior (counts UTF-16 code units).
func padRight(s string, n int) string {
	if utf8.RuneCountInString(s) >= n {
		return s
	}
	return s + repeat(" ", n-utf8.RuneCountInString(s))
}

// padLeft returns s padded with spaces on the left to rune-width n.
// Matches the TypeScript padStart behavior.
func padLeft(s string, n int) string {
	if utf8.RuneCountInString(s) >= n {
		return s
	}
	return repeat(" ", n-utf8.RuneCountInString(s)) + s
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

//	"▶  7. YOU                      156 (YOU)"
//	"   10. Michael Jordan          269"
//
// The exact width and punctuation match the TypeScript output.
func FormatRow(row LeaderboardRow) string {
	tag := "  "
	if row.IsPlayer {
		tag = "▶ "
	}
	rank := padLeft(intToStr(row.SortRank), 2)
	name := padRight(TruncateName(row.Name), NameDisplayMax+2)
	score := padLeft(intToStr(row.Score), 4)
	suffix := ""
	if row.IsPlayer {
		suffix = " (YOU)"
	}
	return tag + rank + ". " + name + score + suffix
}

// intToStr is a tiny strconv-free int formatter so the file does
// not pull in `strconv` for hot path.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
