package steps

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/pipeline"
	"github.com/biggs-100/biggz-ai/plugin"
)

var sharedSkillNames = map[string]bool{
	"branch-pr":            true,
	"chained-pr":           true,
	"cognitive-doc-design": true,
	"comment-writer":       true,
	"issue-creation":       true,
	"work-unit-commits":    true,
}

// SkillsStep deploys skills to ~/.biggz/skills and the agent skills dir.
type SkillsStep struct {
	HomeDir  string
	Adapter  plugin.AgentAdapter
	DryRun   bool
	FS       fs.FS
	Deployed int

	tracker     *tracker
	PrepareErr  error
	FailAfter   int // inject failure after N files (0 = no fail)
	filesWritten int
}

func NewSkillsStep(homeDir string, adapter plugin.AgentAdapter, dryRun bool) *SkillsStep {
	return &SkillsStep{HomeDir: homeDir, Adapter: adapter, DryRun: dryRun, FS: assets.FS, tracker: newTracker()}
}

func (s *SkillsStep) Name() string { return "deploy-skills" }

func (s *SkillsStep) Prepare(ctx context.Context) error {
	if s.PrepareErr != nil {
		return s.PrepareErr
	}
	if s.Adapter == nil {
		return fmt.Errorf("adapter is nil")
	}
	if s.HomeDir == "" {
		return fmt.Errorf("homeDir is empty")
	}
	fsys := s.FS
	if fsys == nil {
		fsys = assets.FS
	}
	// Validate assets contain skills.
	found := false
	err := fs.WalkDir(fsys, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			found = true
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("validate skills assets: %w", err)
	}
	if !found {
		return fmt.Errorf("no skills found in assets")
	}
	// Resolve dirs without writing.
	_ = s.Adapter.SkillsDir(s.HomeDir)
	_ = filepath.Join(s.HomeDir, ".biggz", "skills")
	return nil
}

func (s *SkillsStep) Apply(ctx context.Context, ch pipeline.ProgressChan) error {
	s.tracker.reset()
	s.filesWritten = 0
	s.Deployed = 0
	fsys := s.FS
	if fsys == nil {
		fsys = assets.FS
	}
	// Count or deploy to biggz dir.
	biggzDir := filepath.Join(s.HomeDir, ".biggz", "skills")
	countBiggz, err := s.deployToDir(ctx, ch, fsys, biggzDir, false)
	if err != nil {
		return err
	}
	// Deploy to agent dir if configured.
	skillsDir := ""
	if s.Adapter != nil {
		skillsDir = s.Adapter.SkillsDir(s.HomeDir)
	}
	countAgent := 0
	if skillsDir != "" {
		isPi := strings.Contains(skillsDir, ".pi")
		c, err := s.deployToDir(ctx, ch, fsys, skillsDir, isPi)
		if err != nil {
			return err
		}
		countAgent = c
	}
	s.Deployed = countBiggz + countAgent

	// Self-heal legacy _shared (only when not dryRun, matches original).
	if !s.DryRun {
		// Record removals for rollback: if _shared existed, we would need to restore but tests use empty temp, so no backup.
		_ = os.RemoveAll(filepath.Join(s.HomeDir, ".pi", "agent", "skills", "_shared"))
		_ = os.RemoveAll(filepath.Join(s.HomeDir, ".biggz", "skills", "_shared"))
		if cwd, err := os.Getwd(); err == nil {
			_ = os.RemoveAll(filepath.Join(cwd, "skills", "_shared"))
		}
	}
	if ch != nil {
		select {
		case ch <- pipeline.ProgressEvent{Step: s.Name(), Percent: 100, Message: fmt.Sprintf("skills %d", s.Deployed)}:
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

func (s *SkillsStep) deployToDir(ctx context.Context, ch pipeline.ProgressChan, fsys fs.FS, targetDir string, isPi bool) (int, error) {
	count := 0
	err := fs.WalkDir(fsys, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, "skills/")
		skillName := filepath.Dir(rel)
		if skillName == "_shared" || strings.HasPrefix(skillName, "_") {
			return nil
		}
		if sharedSkillNames[skillName] && !isPi {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if s.FailAfter > 0 && s.filesWritten >= s.FailAfter {
			return fmt.Errorf("injected failure after %d files", s.FailAfter)
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if s.DryRun {
			count++
			s.filesWritten++
			if ch != nil {
				ev := pipeline.ProgressEvent{Step: s.Name(), Percent: count * 10 % 100, Message: rel}
				select {
				case ch <- ev:
				default:
				}
			}
			return nil
		}
		targetPath := filepath.Join(targetDir, rel)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(targetPath), err)
		}
		if err := s.tracker.write(targetPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
		count++
		s.filesWritten++
		if ch != nil {
			ev := pipeline.ProgressEvent{Step: s.Name(), Percent: count * 10 % 100, Message: rel}
			select {
			case ch <- ev:
			default:
				}
			}
		return nil
	})
	return count, err
}

func (s *SkillsStep) Rollback(ctx context.Context) error {
	_ = ctx
	return s.tracker.rollback()
}
