package bigmem

import (
	"testing"
)

// REQ-FTS1: per-token sanitization — every kept token is a quoted literal,
// letterless tokens drop, and the join follows the match mode.
func TestSanitizeFTSTerms(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		mode       string
		wantFTS    string
		wantTokens bool
	}{
		{"hyphen quoted (all)", "gentle-pi", "all", `"gentle-pi"`, true},
		{"hyphen quoted (any)", "gentle-pi", "any", `"gentle-pi"`, true},
		{"hyphen quoted (default mode)", "gentle-pi", "", `"gentle-pi"`, true},
		{"multi-token any joins OR", "gentle-pi ruido", "any", `"gentle-pi" OR "ruido"`, true},
		{"multi-token all joins AND", "gentle-pi ruido", "all", `"gentle-pi" AND "ruido"`, true},
		{"accent kept as literal", "sesión", "all", `"sesión"`, true},
		{"operator words quoted", "AND OR NOT NEAR", "any", `"AND" OR "OR" OR "NOT" OR "NEAR"`, true},
		{"wildcard token dropped", "foo *", "all", `"foo"`, true},
		{"letterless-only drops all", "***", "any", "", false},
		{"blank query yields no tokens", "   ", "all", "", false},
		{"embedded quote stripped", `go"lang`, "all", `"golang"`, true},
		{"digit token kept for rank ordering", "2026", "all", `"2026"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFTS, gotTokens := sanitizeFTSTerms(tt.query, tt.mode)
			if gotFTS != tt.wantFTS || gotTokens != tt.wantTokens {
				t.Errorf("sanitizeFTSTerms(%q, %q) = (%q, %v), want (%q, %v)",
					tt.query, tt.mode, gotFTS, gotTokens, tt.wantFTS, tt.wantTokens)
			}
		})
	}
}

// seedFTSFixtures seeds the hyphen corpus shared by the REQ-FTS1 search tests.
// Hyphen/Ruido carry one token each ("separately"), Combo carries both.
func seedFTSFixtures(t *testing.T, s *Store) {
	t.Helper()
	for _, obs := range []*Observation{
		{Title: "Hyphen note", Content: "marcador gentle-pi unico", Type: "note", Project: "probe"},
		{Title: "Ruido note", Content: "ruido blanco suave", Type: "note", Project: "probe"},
		{Title: "Combo note", Content: "gentle-pi junto a ruido", Type: "note", Project: "probe"},
		{Title: "Accent note", Content: "la sesión de cierre", Type: "note", Project: "probe"},
	} {
		if err := s.Save(obs); err != nil {
			t.Fatalf("save fixture %q: %v", obs.Title, err)
		}
	}
}

func titlesOf(obs []*Observation) map[string]bool {
	titles := make(map[string]bool, len(obs))
	for _, o := range obs {
		titles[o.Title] = true
	}
	return titles
}

// Scenario: Hyphenated any-mode query (REQ-FTS1) — unquoted `gentle-pi OR
// ruido` used to make FTS error ("no such column: pi"), the LIKE fallback
// missed every note and the search returned zero silently.
func TestSearch_FTS_HyphenAnyModeHits(t *testing.T) {
	s := openTestStore(t)
	seedFTSFixtures(t, s)

	got, err := s.Search("gentle-pi ruido", SearchOptions{MatchMode: "any"})
	if err != nil {
		t.Fatalf("any-mode search %q must not error, got %v", "gentle-pi ruido", err)
	}
	titles := titlesOf(got)
	for _, want := range []string{"Hyphen note", "Ruido note", "Combo note"} {
		if !titles[want] {
			t.Errorf("any-mode %q must return %q, got %v", "gentle-pi ruido", want, titles)
		}
	}
}

// All mode keeps AND semantics: only the note carrying both tokens matches.
func TestSearch_FTS_HyphenAllModeRequiresAllTokens(t *testing.T) {
	s := openTestStore(t)
	seedFTSFixtures(t, s)

	got, err := s.Search("gentle-pi ruido", SearchOptions{MatchMode: "all"})
	if err != nil {
		t.Fatalf("all-mode search error: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Combo note" {
		t.Errorf("all-mode %q must return only Combo note, got %v", "gentle-pi ruido", titlesOf(got))
	}
}

// Scenario: Zero-result signal (REQ-FTS1) — a no-match multi-token query must
// come back as an explicit empty set with nil error, never as an FTS parse
// error masked by the LIKE fallback.
func TestSearch_FTS_ZeroResultIsExplicit(t *testing.T) {
	s := openTestStore(t)
	seedFTSFixtures(t, s)

	got, err := s.Search("gentle-pi qqq-inexistente", SearchOptions{MatchMode: "all"})
	if err != nil {
		t.Fatalf("zero-result search must not error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want explicit zero results, got %d", len(got))
	}
}

// Scenario: Accented query (REQ-FTS1) — unicode61 folds diacritics, so both
// the accented and the plain spelling must hit.
func TestSearch_FTS_AccentFolds(t *testing.T) {
	s := openTestStore(t)
	seedFTSFixtures(t, s)

	for _, q := range []string{"sesión", "sesion"} {
		got, err := s.Search(q, SearchOptions{MatchMode: "all"})
		if err != nil {
			t.Fatalf("search %q error: %v", q, err)
		}
		if !titlesOf(got)["Accent note"] {
			t.Errorf("query %q must match the accented note, got %v", q, titlesOf(got))
		}
	}
}

// Scenario: Ordering preserved (REQ-FTS1/REQ-RR2) — non-empty queries keep
// BM25 rank order (even against a more recent row), empty queries keep
// updated_at DESC.
func TestSearch_FTS_OrderingPreserved(t *testing.T) {
	s := openTestStore(t)
	dense := &Observation{Title: "Dense older", Content: "alpha alpha alpha alpha bravo", Type: "note", Project: "probe"}
	sparse := &Observation{Title: "Sparse newer", Content: "alpha charlie delta echo foxtrot golf hotel india juliet kilo lima mike", Type: "note", Project: "probe"}
	if err := s.Save(dense); err != nil {
		t.Fatalf("save dense: %v", err)
	}
	if err := s.Save(sparse); err != nil {
		t.Fatalf("save sparse: %v", err)
	}
	// Backdate the dense row so updated_at DESC would order it LAST: if the
	// non-empty query stopped using rank, the assertion below flips.
	if _, err := s.db.Exec(`UPDATE observations SET updated_at = ? WHERE id = ?`, "2000-01-01T00:00:00Z", dense.ID); err != nil {
		t.Fatalf("backdate dense: %v", err)
	}

	ranked, err := s.Search("alpha", SearchOptions{MatchMode: "all"})
	if err != nil {
		t.Fatalf("rank search: %v", err)
	}
	if len(ranked) != 2 {
		t.Fatalf("want 2 matches, got %d", len(ranked))
	}
	if ranked[0].ID != dense.ID {
		t.Errorf("non-empty query must stay rank-ordered: %q first, want dense %q", ranked[0].Title, dense.Title)
	}

	recent, err := s.Search("", SearchOptions{})
	if err != nil {
		t.Fatalf("empty-query search: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("empty query want 2 rows, got %d", len(recent))
	}
	if recent[0].ID != sparse.ID {
		t.Errorf("empty query must stay updated_at DESC: %q first, want newer %q", recent[0].Title, sparse.Title)
	}
}
