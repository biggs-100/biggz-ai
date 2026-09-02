package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/biggs-100/biggz-ai/internal/pipeline"
	"github.com/biggs-100/biggz-ai/plugintest"
)

func TestSkillsStep_NameStable(t *testing.T) {
	a := &plugintest.FakeAgent{}
	s := NewSkillsStep(t.TempDir(), a, false)
	if s.Name() != s.Name() {
		t.Fatalf("Name not stable")
	}
	if s.Name() != "deploy-skills" {
		t.Fatalf("unexpected name %q", s.Name())
	}
}

func TestSkillsStep_PrepareZeroWrites(t *testing.T) {
	tmp := t.TempDir()
	agent := &plugintest.FakeAgent{}
	agent.SetTempDir(tmp)
	s := NewSkillsStep(tmp, agent, false)
	if err := s.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	// Ensure no files written to tmp after Prepare.
	entries, _ := os.ReadDir(tmp)
	if len(entries) != 0 {
		t.Fatalf("Prepare should write zero files, got %d entries", len(entries))
	}
	// Also ensure no files outside TempDir (can't check easily, but ensure tmp still empty).
	if _, err := os.Stat(filepath.Join(tmp, ".biggz")); err == nil {
		t.Fatalf("Prepare should not create .biggz")
	}
}

func TestSkillsStep_IdempotentMtime(t *testing.T) {
	tmp := t.TempDir()
	agent := &plugintest.FakeAgent{}
	agent.SetTempDir(tmp)
	s := NewSkillsStep(tmp, agent, false)
	// Use real assets.FS; first Apply.
	if err := s.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	ch := make(pipeline.ProgressChan, 32)
	if err := s.Apply(context.Background(), ch); err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	// Find a deployed file.
	biggzSkills := filepath.Join(tmp, ".biggz", "skills")
	// Pick first file from deployed tree.
	var target string
	_ = filepath.Walk(biggzSkills, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && target == "" {
			target = p
		}
		return nil
	})
	if target == "" {
		// fallback to agent skills
		agentSkills := filepath.Join(tmp, "skills")
		_ = filepath.Walk(agentSkills, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && target == "" {
				target = p
			}
			return nil
		})
	}
	if target == "" {
		t.Fatalf("no skill file found after Apply")
	}
	fi1, _ := os.Stat(target)
	mtime1 := fi1.ModTime()
	time.Sleep(10 * time.Millisecond)
	// Second Apply should be idempotent (no mtime change).
	ch2 := make(pipeline.ProgressChan, 32)
	if err := s.Apply(context.Background(), ch2); err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	fi2, _ := os.Stat(target)
	mtime2 := fi2.ModTime()
	if !mtime1.Equal(mtime2) {
		t.Fatalf("idempotent Apply should not change mtime: %v vs %v", mtime1, mtime2)
	}
}

func TestSkillsStep_PartialRollbackCleans(t *testing.T) {
	tmp := t.TempDir()
	agent := &plugintest.FakeAgent{}
	agent.SetTempDir(tmp)
	// Mock FS with 5 skill files (use forward slashes for fstest.MapFS).
	mockFS := fstest.MapFS{}
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("skills/test-skill/file%d.md", i)
		mockFS[name] = &fstest.MapFile{Data: []byte(fmt.Sprintf("content %d", i))}
	}
	// Also add required other skills dirs so Prepare passes? but mockFS only has 5, still found true.
	s := NewSkillsStep(tmp, agent, false)
	s.FS = mockFS
	s.FailAfter = 2
	if err := s.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	ch := make(pipeline.ProgressChan, 32)
	err := s.Apply(context.Background(), ch)
	if err == nil {
		t.Fatalf("expected injected failure")
	}
	// Files 2 should have been written before failure, but partial should be cleaned via explicit Rollback.
	if err := s.Rollback(context.Background()); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	// Check that none of the 5 files exist (since rollback cleans those 2, and remaining 3 never written).
	for i := 0; i < 5; i++ {
		p := filepath.Join(tmp, ".biggz", "skills", "test-skill", "file"+string(rune('0'+i))+".md")
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("file %s should not exist after rollback", p)
		}
	}
	// Rollback idempotent second time.
	if err := s.Rollback(context.Background()); err != nil {
		t.Fatalf("second Rollback should be idempotent, got %v", err)
	}
}

func TestOverlayStep_PrepareZeroWrites(t *testing.T) {
	tmp := t.TempDir()
	agent := &plugintest.FakeAgent{}
	agent.SetTempDir(tmp)
	o := NewOverlayStep(tmp, agent, false)
	if err := o.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	entries, _ := os.ReadDir(tmp)
	if len(entries) != 0 {
		t.Fatalf("Prepare should write zero files, got %d", len(entries))
	}
}

func TestOverlayStep_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	agent := &plugintest.FakeAgent{}
	agent.SetTempDir(tmp)
	o := NewOverlayStep(tmp, agent, false)
	if err := o.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	ch := make(pipeline.ProgressChan, 32)
	if err := o.Apply(context.Background(), ch); err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	// Find settings file.
	settings := agent.SettingsPath(tmp)
	if settings == "" {
		t.Fatalf("no settings path")
	}
	fi1, _ := os.Stat(settings)
	if fi1 == nil {
		t.Fatalf("settings file not created")
	}
	mtime1 := fi1.ModTime()
	time.Sleep(10 * time.Millisecond)
	ch2 := make(pipeline.ProgressChan, 32)
	if err := o.Apply(context.Background(), ch2); err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	fi2, _ := os.Stat(settings)
	if !fi1.ModTime().Equal(fi2.ModTime()) && o.ConfigMerged {
		// If file content identical, mtime should be equal due to WriteFileAtomic idempotent.
		t.Fatalf("overlay idempotent Apply changed mtime: %v vs %v", mtime1, fi2.ModTime())
	}
}

func TestPiExtensionsStep_SkipNonPi(t *testing.T) {
	tmp := t.TempDir()
	agent := &plugintest.FakeAgent{}
	agent.SetTempDir(tmp)
	p := NewPiExtensionsStep(tmp, agent, false)
	if err := p.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare non-pi: %v", err)
	}
	ch := make(pipeline.ProgressChan, 32)
	if err := p.Apply(context.Background(), ch); err != nil {
		t.Fatalf("Apply non-pi: %v", err)
	}
	// Should not create pi dir.
	if _, err := os.Stat(filepath.Join(tmp, ".pi")); err == nil {
		t.Fatalf("non-pi should not create .pi")
	}
}

func TestSteps_FakeAgentTempDirE2E(t *testing.T) {
	tmp := t.TempDir()
	agent := &plugintest.FakeAgent{Installed: true}
	agent.SetTempDir(tmp)
	// Build pipeline via steps.
	skills := NewSkillsStep(tmp, agent, false)
	overlay := NewOverlayStep(tmp, agent, false)
	pi := NewPiExtensionsStep(tmp, agent, false)
	plan := pipeline.NewPlan(skills, overlay, pi)
	orch := &pipeline.Orchestrator{Policy: pipeline.RollbackOnFailure}
	res, err := orch.Run(context.Background(), plan)
	if err != nil {
		t.Fatalf("orch Run: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %v", res.Error)
	}
	// Verify files under tmp (skills + config).
	if _, err := os.Stat(filepath.Join(tmp, ".biggz", "skills")); err != nil {
		t.Fatalf("skills under tmp not found: %v", err)
	}
	if _, err := os.Stat(agent.SettingsPath(tmp)); err != nil {
		t.Fatalf("settings under tmp not found: %v", err)
	}
	// Ensure no file outside tmp (check real home not polluted - we use tmp as home, so parent of tmp should not have .biggz).
	parent := filepath.Dir(tmp)
	if _, err := os.Stat(filepath.Join(parent, ".biggz", "skills")); err == nil {
		t.Fatalf("should not write outside TempDir")
	}
}

func TestOrchestrator_RollbackPartialSteps(t *testing.T) {
	tmp := t.TempDir()
	agent := &plugintest.FakeAgent{}
	agent.SetTempDir(tmp)
	s1 := NewSkillsStep(tmp, agent, false)
	s2 := NewOverlayStep(tmp, agent, false)
	failing := &failingStep{name: "fail-step", failAfter: 0}
	plan := pipeline.NewPlan(s1, failing, s2)
	orch := &pipeline.Orchestrator{Policy: pipeline.RollbackOnFailure}
	res, err := orch.Run(context.Background(), plan)
	if err == nil {
		t.Fatalf("expected failure")
	}
	if res.Success {
		t.Fatalf("expected not success")
	}
	// s1 should have been rolled back (its files removed). Check recursively that no files remain.
	if _, err := os.Stat(filepath.Join(tmp, ".biggz", "skills")); err == nil {
		fileCount := 0
		_ = filepath.Walk(filepath.Join(tmp, ".biggz", "skills"), func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				fileCount++
			}
			return nil
		})
		if fileCount != 0 {
			t.Fatalf("s1 should be rolled back, but %d files remain", fileCount)
		}
	}
	_ = s2
}

func TestSkillsStep_BurstProgress(t *testing.T) {
	tmp := t.TempDir()
	agent := &plugintest.FakeAgent{}
	agent.SetTempDir(tmp)
	s := NewSkillsStep(tmp, agent, false)
	ch := make(pipeline.ProgressChan, 32)
	// Run Apply in goroutine and drain.
	done := make(chan int, 1)
	go func() {
		_ = s.Prepare(context.Background())
		_ = s.Apply(context.Background(), ch)
		close(ch)
	}()
	count := 0
	for range ch {
		count++
	}
	// Should have at least some events (skills count).
	if count == 0 {
		t.Fatalf("expected progress events, got 0")
	}
	done <- count
}

type failingStep struct {
	name      string
	failAfter int
	called    int
}

func (f *failingStep) Name() string { return f.name }
func (f *failingStep) Prepare(ctx context.Context) error { return nil }
func (f *failingStep) Apply(ctx context.Context, ch pipeline.ProgressChan) error {
	f.called++
	return fmt.Errorf("injected failure")
}
func (f *failingStep) Rollback(ctx context.Context) error { return nil }
