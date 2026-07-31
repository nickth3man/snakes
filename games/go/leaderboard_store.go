//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"
)

// leaderboard_store.go — localStorage IO boundary for the leaderboard.
// Mirrors games/typescript/src/leaderboard/leaderboard-store.ts.
//
// NPC entries are generated once from the fixed seed and persisted in
// localStorage; the player row is merged live at every read from the
// best-score key (already owned by main.go as `snake-go-best`), so it
// is never stale and never rewritten. Safari private mode: setItem
// throws → caught → in-memory memo only.
//
// All js.Global() / js.Value() work is confined to this file. Pure
// logic (merge, format, truncate, RNG) lives in leaderboard.go and is
// host-testable without the Go runtime.

const (
	lbKey     = "snake-leaderboard" // NPC entries doc
	lbBestKey = "snake-go-best"     // READ-ONLY — owned by main.go
	lbDocVer  = 1
)

// lbDoc is the persisted shape. Player is NOT stored here.
type lbDoc struct {
	Version     int        `json:"version"`
	Seed        uint32     `json:"seed"`
	GeneratedAt int64      `json:"generatedAt"`
	Entries     []NpcEntry `json:"entries"`
}

// memo: localStorage is hit at most once per session.
var lbMemo *lbDoc

func lbReadBestScore() int {
	defer func() { _ = recover() }()
	ls := js.Global().Get("localStorage")
	if ls.IsUndefined() || ls.IsNull() {
		return 0
	}
	raw := ls.Call("getItem", lbBestKey)
	if raw.IsNull() || raw.IsUndefined() {
		return 0
	}
	s := raw.String()
	if s == "" {
		return 0
	}
	n, err := parseInt(s, 10)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func lbLoadDoc() *lbDoc {
	if lbMemo != nil {
		return lbMemo
	}
	ls := js.Global().Get("localStorage")
	var doc *lbDoc
	if !(ls.IsUndefined() || ls.IsNull()) {
		func() {
			defer func() { _ = recover() }()
			raw := ls.Call("getItem", lbKey)
			if !raw.IsNull() && !raw.IsUndefined() {
				s := raw.String()
				if s != "" {
					var d lbDoc
					if json.Unmarshal([]byte(s), &d) == nil {
						doc = &d
					}
				}
			}
		}()
	}

	if doc == nil ||
		doc.Version != lbDocVer ||
		doc.Seed != LeaderboardSeed ||
		len(doc.Entries) == 0 {
		doc = &lbDoc{
			Version:     lbDocVer,
			Seed:        LeaderboardSeed,
			GeneratedAt: jsDateNow(),
			Entries:     GenerateNpcEntries(LeaderboardSeed),
		}
		if !(ls.IsUndefined() || ls.IsNull()) {
			func() {
				defer func() { _ = recover() }() // Safari private mode
				b, _ := json.Marshal(doc)
				ls.Call("setItem", lbKey, string(b))
			}()
		}
	}
	lbMemo = doc
	return doc
}

// lbGetLeaderboard is the public entry point for the UI. Cheap and
// safe to call per session.
func lbGetLeaderboard() []LeaderboardRow {
	doc := lbLoadDoc()
	best := lbReadBestScore()
	var p *PlayerEntry
	if best > 0 {
		p = &PlayerEntry{Score: best}
	}
	return MergeWithPlayer(doc.Entries, p)
}

// lbResetMemo is for tests / hot reload. Not exported to JS.
func lbResetMemo() {
	lbMemo = nil
}

// jsDateNow returns Date.now() from JS, or 0 if unavailable.
func jsDateNow() int64 {
	d := js.Global().Get("Date")
	if d.IsUndefined() || d.IsNull() {
		return 0
	}
	return int64(d.Call("now").Float())
}

// parseInt is a tiny strconv-free parser for the few ints we need
// from localStorage (avoids dragging strconv into the wasm binary).
func parseInt(s string, base int) (int, error) {
	if base != 10 {
		// The original parseInt(s, 10) usage is the only path; base != 10
		// is unreachable here but kept for completeness.
		return 0, errParseInt
	}
	n, ok := parseInt10(s)
	if !ok {
		return 0, errParseInt
	}
	return n, nil
}

var errParseInt = parseIntError{}

type parseIntError struct{}

func (parseIntError) Error() string { return "parseInt: invalid" }

func parseInt10(s string) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}
	i := 0
	neg := false
	if s[0] == '-' {
		neg = true
		i = 1
	} else if s[0] == '+' {
		i = 1
	}
	if i == len(s) {
		return 0, false
	}
	n := 0
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
		if n > 1<<31-1 {
			return 0, false
		}
	}
	if neg {
		n = -n
	}
	return n, true
}
