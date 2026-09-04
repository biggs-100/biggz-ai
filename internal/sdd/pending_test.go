package sdd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
)

func eqPending(a, b PendingQuestion) bool { ab, _ := json.Marshal(a); bb, _ := json.Marshal(b); return string(ab) == string(bb) }
func setupPendingWS(t *testing.T, ws, ch string) { t.Helper(); _ = os.MkdirAll(filepath.Join(ws, "openspec", "changes", ch), 0o755); _ = os.WriteFile(filepath.Join(ws, "openspec", "changes", ch, "state.yaml"), []byte("phases:\n  propose: pending\n"), 0o644) }

func TestPendingDualWriteEquality(t *testing.T) {
	ws, sr := t.TempDir(), t.TempDir()
	SetBigMemStoreRootForTest(sr)
	defer SetBigMemStoreRootForTest("")
	ch := "test-pending"
	setupPendingWS(t, ws, ch)
	pq := PendingQuestion{Schema: PendingSchema, Change: ch, Envelope: QuestionEnvelope{Questions: []Question{{Header: "Hdr1", Question: "Q1?", Options: []QuestionOption{{Label: "proceed", Description: "continue as planned"}, {Label: "adjust", Description: "change direction first"}, {Label: "stop", Description: "halt here"}}}}}, SynthesisMD: "## Sub-agent Result: test\n**What was done:** did\n**Artifacts/Paths:** a/b, c/d\n**Risks / Open Questions:** none\n**Next Recommended:** next\n"}
	if err := SavePendingDualWriteAt(ws, ch, pq); err != nil {
		t.Fatalf("save: %v", err)
	}
	if ok, err := VerifyEqualityAt(ws, ch); err != nil || !ok {
		t.Fatalf("equality %v %v", ok, err)
	}
	bm, _ := loadPendingFromBigMem(ws, ch)
	fs, _ := readPendingFromState(ws, ch)
	if !eqPending(bm, fs) {
		t.Fatalf("bm/fs mismatch")
	}
	if bm.Schema != PendingSchema {
		t.Errorf("schema %q", bm.Schema)
	}
}

func TestPendingCompactionFallback(t *testing.T) {
	ws, sr := t.TempDir(), t.TempDir()
	SetBigMemStoreRootForTest(sr)
	defer SetBigMemStoreRootForTest("")
	ch := "test-pending-fallback"
	setupPendingWS(t, ws, ch)
	pq := PendingQuestion{Schema: PendingSchema, Change: ch, Envelope: QuestionEnvelope{Questions: []Question{{Header: "Checkpoint", Question: "Next?", Options: []QuestionOption{{Label: "proceed", Description: "continue as planned"}, {Label: "adjust", Description: "change direction first"}, {Label: "stop", Description: "halt here"}}}, {Header: "Prefs", Question: "Which?", Options: []QuestionOption{{Label: "a", Description: "first"}, {Label: "b", Description: "second"}}}}}, SynthesisMD: "## Sub-agent Result: x\n**What was done:** y\n**Artifacts/Paths:** p/q\n**Risks / Open Questions:** none\n**Next Recommended:** verify\n"}
	if err := SavePendingDualWriteAt(ws, ch, pq); err != nil {
		t.Fatalf("save: %v", err)
	}
	store, _ := bigmem.Open(sr)
	results, _ := store.Search(pendingTopicKey(ch), bigmem.SearchOptions{Limit: 1})
	for _, r := range results {
		if strings.EqualFold(r.TopicKey, pendingTopicKey(ch)) {
			_ = store.Delete(r.ID)
		}
	}
	store.Close()
	loaded, err := LoadOnCompactionAt(ws, ch)
	if err != nil {
		t.Fatalf("load fallback: %v", err)
	}
	if !eqPending(loaded, pq) && len(loaded.Envelope.Questions) != len(pq.Envelope.Questions) {
		t.Fatalf("fallback mismatch")
	}
	md := PendingFallbackMD(loaded)
	if !strings.Contains(md, "Checkpoint") || !strings.Contains(md, "proceed") {
		t.Errorf("fallback md %q", md)
	}
	orig, _ := os.Getwd()
	_ = os.Chdir(ws)
	defer os.Chdir(orig)
	if md2, err := LoadPendingFallback(ch); err != nil || !strings.Contains(md2, "proceed") {
		t.Fatalf("LoadPendingFallback %v %q", err, md2)
	}
}

func TestReadLoopLarge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.md")
	size, content := 70*1024, strings.Repeat("a", 70*1024)
	_ = os.WriteFile(path, []byte(content), 0o644)
	got, err := ReadLoop(path, 50*1024)
	if err != nil || len(got) != size || got != content {
		t.Fatalf("ReadLoop %v len %d", err, len(got))
	}
	fn := func(off, lim int) (string, error) {
		if off >= len(content) {
			return "", nil
		}
		end := off + lim
		if end > len(content) {
			end = len(content)
		}
		return content[off:end], nil
	}
	if got2, err := ReadLoopWithFunc(fn, size); err != nil || got2 != content {
		t.Fatalf("withFunc %v", err)
	}
	ws, sr := t.TempDir(), t.TempDir()
	SetBigMemStoreRootForTest(sr)
	defer SetBigMemStoreRootForTest("")
	ch := "large-pending"
	setupPendingWS(t, ws, ch)
	large := "## Sub-agent Result: large\n**What was done:** x\n**Artifacts/Paths:** " + strings.Repeat("a/b, ", 200) + "\n**Risks / Open Questions:** none\n**Next Recommended:** next\n**Preview:** " + strings.Repeat("p", 60*1024) + "\n"
	pq := PendingQuestion{Schema: PendingSchema, Change: ch, Envelope: QuestionEnvelope{Questions: []Question{{Header: "H", Question: "Q?", Options: []QuestionOption{{Label: "proceed", Description: "continue as planned"}, {Label: "stop", Description: "halt here"}}}}}, SynthesisMD: large}
	if err := SavePendingDualWriteAt(ws, ch, pq); err != nil {
		t.Fatalf("save large %v", err)
	}
	if ok, _ := VerifyEqualityAt(ws, ch); !ok {
		t.Fatalf("large not equal")
	}
}
