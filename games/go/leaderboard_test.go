package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goldenFixture mirrors the JSON structure produced by
// games/typescript/scripts/gen-leaderboard-golden.mts.
type goldenFixture struct {
	Version       int                 `json:"version"`
	Seed          uint32              `json:"seed"`
	NameCount     int                 `json:"name_count"`
	ScoreMax      int                 `json:"score_max"`
	ScoreMin      int                 `json:"score_min"`
	CurveExponent float64             `json:"curve_exponent"`
	JitterAmp     int                 `json:"jitter_amp"`
	NameDisplay   int                 `json:"name_display_max"`
	NPCScores     []goldenNPC         `json:"npc_scores"`
	Cases         map[string][]string `json:"cases"`
}

type goldenNPC struct {
	Rank  int `json:"rank"`
	Score int `json:"score"`
}

func loadFixture(t *testing.T) goldenFixture {
	t.Helper()
	p, err := filepath.Abs("../../testdata/leaderboard-golden.json")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read fixture %s: %v", p, err)
	}
	var f goldenFixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if f.Version != 1 || f.Seed != LeaderboardSeed || f.NameCount != NameCount {
		t.Fatalf("fixture header mismatch: got %+v", f)
	}
	return f
}

// TestMulberry32ParityAgainstTypeScript verifies that the Go
// mulberry32 port produces byte-for-byte identical output to the
// TypeScript implementation in games/typescript/src/benchmark/rng.ts.
func TestMulberry32ParityAgainstTypeScript(t *testing.T) {
	want := []float64{
		0.6011037519201636,
		0.44829055899754167,
		0.8524657934904099,
		0.6697340414393693,
		0.17481389874592423,
		0.5265925421845168,
		0.2732279943302274,
		0.6247446539346129,
		0.8654746483080089,
		0.4723170551005751,
	}
	rng := mulberry32(42)
	for i, w := range want {
		got := rng()
		if got != w {
			t.Errorf("mulberry32(42) call %d:\n got %.16f\nwant %.16f", i, got, w)
		}
	}
}

func TestScoreCurveBase(t *testing.T) {
	if got := ScoreCurveBase(1); got < ScoreMax-JitterAmp || got > ScoreMax {
		t.Errorf("rank 1 base %d not in [%d, %d]", got, ScoreMax-JitterAmp, ScoreMax)
	}
	if got := ScoreCurveBase(NameCount); got < ScoreMin || got > ScoreMin+JitterAmp {
		t.Errorf("rank %d base %d not in [%d, %d]", NameCount, got, ScoreMin, ScoreMin+JitterAmp)
	}
	prev := ScoreCurveBase(1) + 1
	for i := 1; i <= NameCount; i++ {
		got := ScoreCurveBase(i)
		if got > prev {
			t.Errorf("rank %d not monotonic: %d > prev %d", i, got, prev)
		}
		prev = got
	}
}

func TestScoreCurveBaseEndpoints(t *testing.T) {
	if got := ScoreCurveBase(1); got != ScoreMax {
		t.Errorf("rank 1 exact base should be %d, got %d", ScoreMax, got)
	}
	if got := ScoreCurveBase(NameCount); got != ScoreMin {
		t.Errorf("rank %d exact base should be %d, got %d", NameCount, ScoreMin, got)
	}
}

func TestGenerateNpcEntriesDeterministic(t *testing.T) {
	a := GenerateNpcEntries(LeaderboardSeed)
	b := GenerateNpcEntries(LeaderboardSeed)
	if len(a) != NameCount || len(b) != NameCount {
		t.Fatalf("length mismatch: %d, %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Score != b[i].Score || a[i].Name != b[i].Name || a[i].Rank != b[i].Rank {
			t.Errorf("entry %d mismatch: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestGenerateNpcEntriesLength(t *testing.T) {
	out := GenerateNpcEntries(LeaderboardSeed)
	if len(out) != NameCount {
		t.Fatalf("expected %d, got %d", NameCount, len(out))
	}
}

func TestNamesMatchTypeScript(t *testing.T) {
	f := loadFixture(t)
	if len(NAMES) != NameCount {
		t.Fatalf("NAMES length %d, expected %d", len(NAMES), NameCount)
	}
	out := GenerateNpcEntries(LeaderboardSeed)
	for i, want := range f.NPCScores {
		if out[i].Rank != want.Rank || out[i].Score != want.Score {
			t.Errorf("rank %d: got score=%d, want %d", want.Rank, out[i].Score, want.Score)
		}
	}
}

func TestMergeWithPlayerNoPlayer(t *testing.T) {
	npc := GenerateNpcEntries(LeaderboardSeed)
	rows := MergeWithPlayer(npc, nil)
	if len(rows) != NameCount {
		t.Fatalf("expected %d, got %d", NameCount, len(rows))
	}
	for i, r := range rows {
		if r.SortRank != i+1 {
			t.Errorf("row %d sortRank %d", i, r.SortRank)
		}
		if r.IsPlayer {
			t.Errorf("row %d should not be player", i)
		}
	}
}

func TestMergeWithPlayerHighScore(t *testing.T) {
	npc := GenerateNpcEntries(LeaderboardSeed)
	rows := MergeWithPlayer(npc, &PlayerEntry{Score: 500})
	if len(rows) != NameCount+1 {
		t.Fatalf("expected %d, got %d", NameCount+1, len(rows))
	}
	if rows[0].SortRank != 1 || !rows[0].IsPlayer {
		t.Errorf("expected player at rank 1, got %+v", rows[0])
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].SortRank != i+1 {
			t.Errorf("non-player row %d sortRank %d", i, rows[i].SortRank)
		}
	}
}

func TestMergeWithPlayerTieBreaker(t *testing.T) {
	npc := GenerateNpcEntries(LeaderboardSeed)
	target := npc[10].Score
	rows := MergeWithPlayer(npc, &PlayerEntry{Score: target})
	for i, r := range rows {
		if r.IsPlayer {
			if i+1 >= len(rows) || rows[i+1].Score != target {
				t.Errorf("player not placed above tied score; rows: %+v", rows[:i+2])
			}
			return
		}
	}
	t.Fatal("player not present")
}

func TestMergeWithPlayerZeroScore(t *testing.T) {
	npc := GenerateNpcEntries(LeaderboardSeed)
	rows := MergeWithPlayer(npc, &PlayerEntry{Score: 0})
	if len(rows) != NameCount {
		t.Errorf("zero-score player should not appear, got %d rows", len(rows))
	}
	for _, r := range rows {
		if r.IsPlayer {
			t.Errorf("zero-score player should not appear, got %+v", r)
		}
	}
}

func TestTruncateName(t *testing.T) {
	if got := TruncateName("Zeus"); got != "Zeus" {
		t.Errorf("short name: %q", got)
	}
	long := "Dominique 'The Human Highlight Film' Wilkins"
	utf8Len := func(s string) int {
		n := 0
		for range s {
			n++
		}
		return n
	}
	if utf8Len(long) <= NameDisplayMax {
		t.Fatalf("test setup: long name must be > %d runes", NameDisplayMax)
	}
	got := TruncateName(long)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("long name should end with ellipsis: %q", got)
	}
	if utf8Len(got) != NameDisplayMax {
		t.Errorf("long name rune length %d, expected %d", utf8Len(got), NameDisplayMax)
	}
}

func TestFormatRowPlayer(t *testing.T) {
	r := LeaderboardRow{SortRank: 7, Score: 156, IsPlayer: true, Name: "YOU"}
	want := "▶  7. YOU                      156 (YOU)"
	if got := FormatRow(r); got != want {
		t.Errorf("player row:\n got %q\nwant %q", got, want)
	}
}

func TestFormatRowNPC(t *testing.T) {
	r := LeaderboardRow{SortRank: 10, Score: 269, IsPlayer: false, Name: "Michael Jordan"}
	want := "  10. Michael Jordan           269"
	if got := FormatRow(r); got != want {
		t.Errorf("npc row:\n got %q\nwant %q", got, want)
	}
}

func TestFormatRowAllRowsLength(t *testing.T) {
	npc := GenerateNpcEntries(LeaderboardSeed)
	rows := MergeWithPlayer(npc, &PlayerEntry{Score: 156})
	for _, r := range rows {
		s := FormatRow(r)
		maxLen := 42
		if len(s) > maxLen {
			t.Errorf("row %q too long (%d > %d)", s, len(s), maxLen)
		}
	}
}

// TestGoldenParity loads the TypeScript-generated golden fixture
// and checks every byte of every formatRow output. If this passes,
// the Go leaderboard matches the TypeScript source exactly.
func TestGoldenParity(t *testing.T) {
	f := loadFixture(t)
	npc := GenerateNpcEntries(LeaderboardSeed)

	cases := []struct {
		name   string
		player *PlayerEntry
	}{
		{"no_player", nil},
		{"low_best", &PlayerEntry{Score: 50}},
		{"mid_best", &PlayerEntry{Score: 156}},
		{"top_best", &PlayerEntry{Score: 500}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows := MergeWithPlayer(npc, c.player)
			got := make([]string, len(rows))
			for i, r := range rows {
				got[i] = FormatRow(r)
			}
			want, ok := f.Cases[c.name]
			if !ok {
				t.Fatalf("fixture has no case %q", c.name)
			}
			if len(got) != len(want) {
				t.Fatalf("case %q: row count %d != fixture %d", c.name, len(got), len(want))
			}
			for i := range got {
				if got[i] != want[i] {
					t.Errorf("case %q row %d:\n got %q\nwant %q", c.name, i, got[i], want[i])
				}
			}
		})
	}
}
