package bigmem

// Permanent tests for the context-aware Store API (CTX-1..CTX-4, SDD
// fix-bigmem-store-ctx Phase 4). Table-driven cancelled-ctx coverage for all
// 8 *Ctx methods, legacy parity, WithTimeout default-vs-override semantics,
// and temp-DB integration (round-trip, WAL contention, fast pre-check fail).

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func cancelledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// TestCtxCancelledTable proves every *Ctx twin fails visibly (wrapping
// ctx.Err(), zero silent fallback) when the caller ctx is already cancelled.
func TestCtxCancelledTable(t *testing.T) {
	s := openTestStore(t)
	seed := &Observation{Title: "seed", Type: "decision", Content: "seed content", Project: "test"}
	if err := s.Save(seed); err != nil {
		t.Fatalf("seed Save() error: %v", err)
	}
	ctx := cancelledCtx()
	cases := []struct {
		name string
		call func() error
	}{
		{"SaveCtx", func() error {
			return s.SaveCtx(ctx, &Observation{Title: "x", Type: "decision", Content: "x"})
		}},
		{"GetCtx", func() error {
			_, err := s.GetCtx(ctx, seed.ID)
			return err
		}},
		{"SearchCtx", func() error {
			_, err := s.SearchCtx(ctx, "seed", SearchOptions{})
			return err
		}},
		{"UpdateCtx", func() error {
			_, err := s.UpdateCtx(ctx, seed.ID, map[string]any{"title": "y"})
			return err
		}},
		{"DeleteCtx", func() error { return s.DeleteCtx(ctx, seed.ID) }},
		{"SessionContextCtx", func() error {
			_, err := s.SessionContextCtx(ctx, 5)
			return err
		}},
		{"TimelineCtx", func() error {
			_, err := s.TimelineCtx(ctx, TimelineOptions{Limit: 5})
			return err
		}},
		{"SavePromptCtx", func() error {
			_, err := s.SavePromptCtx(ctx, "prompt", "sess-1")
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatalf("%s with cancelled ctx: expected error, got nil", tc.name)
			}
			if !errors.Is(err, context.Canceled) {
				t.Errorf("%s: error %v does not wrap context.Canceled", tc.name, err)
			}
		})
	}
}

// TestCtxParity proves each *Ctx twin matches its legacy wrapper on the happy
// path (CTX-4 wrapper-parity scenario).
func TestCtxParity(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	obs := &Observation{Title: "parity", Type: "architecture", Content: "ctx content", Project: "test"}
	if err := s.SaveCtx(ctx, obs); err != nil {
		t.Fatalf("SaveCtx() error: %v", err)
	}
	gotCtx, err := s.GetCtx(ctx, obs.ID)
	if err != nil {
		t.Fatalf("GetCtx() error: %v", err)
	}
	gotLegacy, err := s.Get(obs.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if gotCtx.Title != gotLegacy.Title || gotCtx.Content != gotLegacy.Content {
		t.Errorf("GetCtx/Legacy mismatch: %+v vs %+v", gotCtx, gotLegacy)
	}

	resCtx, err := s.SearchCtx(ctx, "parity", SearchOptions{})
	if err != nil {
		t.Fatalf("SearchCtx() error: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	resLegacy, err := s.Search("parity", SearchOptions{})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(resCtx) != len(resLegacy) {
		t.Errorf("SearchCtx=%d results, Search=%d results", len(resCtx), len(resLegacy))
	}

	updCtx, err := s.UpdateCtx(ctx, obs.ID, map[string]any{"title": "parity-v2"})
	if err != nil {
		t.Fatalf("UpdateCtx() error: %v", err)
	}
	if updCtx.Title != "parity-v2" {
		t.Errorf("UpdateCtx title = %q, want parity-v2", updCtx.Title)
	}
	if _, err := s.SessionStart("sess-ctx-1", "test"); err != nil {
		t.Fatalf("SessionStart error: %v", err)
	}
	sessCtx, err := s.SessionContextCtx(ctx, 5)
	if err != nil {
		t.Fatalf("SessionContextCtx() error: %v", err)
	}
	sessLegacy, err := s.SessionContext(5)
	if err != nil {
		t.Fatalf("SessionContext() error: %v", err)
	}
	if len(sessCtx) != len(sessLegacy) {
		t.Errorf("SessionContextCtx=%d sessions, SessionContext=%d", len(sessCtx), len(sessLegacy))
	}
	tlCtx, err := s.TimelineCtx(ctx, TimelineOptions{Limit: 10})
	if err != nil {
		t.Fatalf("TimelineCtx() error: %v", err)
	}
	tlLegacy, err := s.Timeline(TimelineOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Timeline() error: %v", err)
	}
	if len(tlCtx) != len(tlLegacy) {
		t.Errorf("TimelineCtx=%d entries, Timeline=%d", len(tlCtx), len(tlLegacy))
	}
	pCtx, err := s.SavePromptCtx(ctx, "parity prompt", "sess-ctx-1")
	if err != nil {
		t.Fatalf("SavePromptCtx() error: %v", err)
	}
	if pCtx.ID == "" {
		t.Error("SavePromptCtx returned empty ID")
	}
	if err := s.DeleteCtx(ctx, obs.ID); err != nil {
		t.Fatalf("DeleteCtx() error: %v", err)
	}
	if _, err := s.Get(obs.ID); err == nil {
		t.Error("expected Get() error after DeleteCtx")
	}
}

// TestWithTimeoutDefaultVsOverride proves CTX-3: a caller ctx without
// deadline gets the 5s default, while a caller-supplied deadline always wins
// and is never extended.
func TestWithTimeoutDefaultVsOverride(t *testing.T) {
	before := time.Now()
	ctx, cancel := WithTimeout(context.Background())
	defer cancel()
	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("WithTimeout(Background): expected a default deadline, got none")
	}
	if remaining := time.Until(dl); remaining <= 0 || remaining > defaultBigmemTimeout {
		t.Errorf("default deadline in %v, want (0, %v]", remaining, defaultBigmemTimeout)
	}
	if dl.Before(before) {
		t.Errorf("default deadline %v is before test start %v", dl, before)
	}

	short, cancelShort := context.WithTimeout(context.Background(), time.Second)
	defer cancelShort()
	child, cancelChild := WithTimeout(short)
	defer cancelChild()
	childDl, ok := child.Deadline()
	if !ok {
		t.Fatal("WithTimeout(short): expected caller deadline to survive, got none")
	}
	shortDl, _ := short.Deadline()
	if !childDl.Equal(shortDl) {
		t.Errorf("caller deadline overridden: child=%v caller=%v", childDl, shortDl)
	}
}

// TestSearchCtxWALContention is the temp-DB integration test: concurrent
// *Ctx writers plus SearchCtx readers under contention must all succeed with
// live contexts (CTX-4 driver-cancellation visibility, no hangs).
func TestSearchCtxWALContention(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				o := &Observation{
					Title:   fmt.Sprintf("contention-%d-%d", w, i),
					Type:    "decision",
					Content: fmt.Sprintf("writer %d op %d content", w, i),
					Project: "test",
				}
				if err := s.SaveCtx(ctx, o); err != nil {
					errs <- err
					return
				}
			}
		}(w)
	}
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				if _, err := s.SearchCtx(ctx, "contention", SearchOptions{Limit: 10}); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("WAL contention op failed: %v", err)
	}
	got, err := s.Search("contention", SearchOptions{Limit: 50})
	if err != nil {
		t.Fatalf("final Search() error: %v", err)
	}
	if len(got) != 40 {
		t.Errorf("expected 40 contended observations, got %d", len(got))
	}
}

// TestCtxCancelledFastFail proves the ctx.Done pre-check short-circuits
// before SQLite: even against a closed store, a cancelled ctx surfaces
// context.Canceled instead of a driver error.
func TestCtxCancelledFastFail(t *testing.T) {
	s := openTestStore(t)
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	ctx := cancelledCtx()
	if _, err := s.SearchCtx(ctx, "anything", SearchOptions{}); !errors.Is(err, context.Canceled) {
		t.Errorf("SearchCtx on closed store with cancelled ctx = %v, want context.Canceled", err)
	}
	if _, err := s.GetCtx(ctx, "missing"); !errors.Is(err, context.Canceled) {
		t.Errorf("GetCtx on closed store with cancelled ctx = %v, want context.Canceled", err)
	}
}
