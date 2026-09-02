package steps

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/internal/pipeline"
	"github.com/biggs-100/biggz-ai/plugin"
)

type OverlayStep struct {
	HomeDir         string
	Adapter         plugin.AgentAdapter
	DryRun          bool
	FS              fs.FS
	ConfigMerged    bool
	CommandsWritten int
	PluginsDeployed int
	PromptsDeployed int
	tracker         *tracker
	PrepareErr      error
	FailAfter       int
}

func NewOverlayStep(homeDir string, adapter plugin.AgentAdapter, dryRun bool) *OverlayStep {
	return &OverlayStep{HomeDir: homeDir, Adapter: adapter, DryRun: dryRun, FS: assets.FS, tracker: newTracker()}
}
func (o *OverlayStep) Name() string { return "deploy-overlay" }
func (o *OverlayStep) Prepare(ctx context.Context) error {
	if o.PrepareErr != nil {
		return o.PrepareErr
	}
	if o.Adapter == nil {
		return fmt.Errorf("adapter is nil")
	}
	if o.HomeDir == "" {
		return fmt.Errorf("homeDir empty")
	}
	fsys := o.FS
	if fsys == nil {
		fsys = assets.FS
	}
	if _, err := fs.ReadFile(fsys, "opencode/sdd-overlay-multi.json"); err != nil {
		return fmt.Errorf("overlay asset missing: %w", err)
	}
	_ = o.Adapter.SettingsPath(o.HomeDir)
	return nil
}
func (o *OverlayStep) Apply(ctx context.Context, ch pipeline.ProgressChan) error {
	o.tracker.reset()
	o.ConfigMerged = false
	o.CommandsWritten = 0
	o.PluginsDeployed = 0
	o.PromptsDeployed = 0
	fsys := o.FS
	if fsys == nil {
		fsys = assets.FS
	}
	merged, err := o.deployConfig(ctx, fsys)
	if err != nil {
		return err
	}
	o.ConfigMerged = merged
	if ch != nil {
		select {
		case ch <- pipeline.ProgressEvent{Step: o.Name(), Percent: 20, Message: "config merged"}:
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	if o.Adapter.ID() != "pi" {
		if err := o.deployPrompts(ctx, fsys); err != nil {
			return err
		}
	}
	if !(o.Adapter.ID() == "pi" && !o.Adapter.SupportsSlashCommands()) {
		n, err := o.deployCommands(ctx, fsys)
		if err != nil {
			return err
		}
		o.CommandsWritten = n
	}
	if o.Adapter.ID() != "pi" {
		n, err := o.deployPlugins(ctx, fsys)
		if err != nil {
			return err
		}
		o.PluginsDeployed = n
	}
	if err := o.deployPersona(ctx, fsys); err != nil {
		return err
	}
	if err := o.deployBigMemProtocol(ctx, fsys); err != nil {
		return err
	}
	if ch != nil {
		select {
		case ch <- pipeline.ProgressEvent{Step: o.Name(), Percent: 100, Message: "overlay done"}:
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}
func (o *OverlayStep) deployConfig(ctx context.Context, fsys fs.FS) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	settingsPath := o.Adapter.SettingsPath(o.HomeDir)
	if settingsPath == "" {
		return false, nil
	}
	overlay, err := generateOverlay(fsys, o.HomeDir)
	if err != nil {
		return false, err
	}
	var existing []byte
	if _, err := os.Stat(settingsPath); err == nil {
		existing, err = os.ReadFile(settingsPath)
		if err != nil {
			return false, err
		}
	}
	var merged []byte
	if len(existing) > 0 {
		merged, err = filemerge.MergeJSONC(existing, overlay)
		if err != nil {
			merged, err = filemerge.MergeJSONC([]byte("{}"), overlay)
			if err != nil {
				return false, err
			}
		}
	} else {
		merged, err = filemerge.MergeJSONC([]byte("{}"), overlay)
		if err != nil {
			return false, err
		}
	}
	if o.DryRun {
		return true, nil
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return false, err
	}
	if err := o.tracker.write(settingsPath, merged, 0644); err != nil {
		return false, err
	}
	return true, nil
}
func (o *OverlayStep) deployPrompts(ctx context.Context, fsys fs.FS) error {
	promptsDir := filepath.Join(o.Adapter.GlobalConfigDir(o.HomeDir), "prompts", "sdd")
	count := 0
	return fs.WalkDir(fsys, "prompts/sdd", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if o.FailAfter > 0 && count >= o.FailAfter {
			return fmt.Errorf("injected failure after %d prompts", o.FailAfter)
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		rel := path[len("prompts/sdd/"):]
		target := filepath.Join(promptsDir, rel)
		if o.DryRun {
			count++
			o.PromptsDeployed++
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := o.tracker.write(target, data, 0644); err != nil {
			return err
		}
		count++
		o.PromptsDeployed++
		return nil
	})
}
func (o *OverlayStep) deployCommands(ctx context.Context, fsys fs.FS) (int, error) {
	commandsDir := filepath.Join(o.Adapter.GlobalConfigDir(o.HomeDir), "commands")
	count := 0
	err := fs.WalkDir(fsys, "opencode/commands", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if o.FailAfter > 0 && count >= o.FailAfter {
			return fmt.Errorf("injected failure after %d commands", o.FailAfter)
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		rel := path[len("opencode/commands/"):]
		target := filepath.Join(commandsDir, rel)
		if o.DryRun {
			count++
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := o.tracker.write(target, data, 0644); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}
func (o *OverlayStep) deployPlugins(ctx context.Context, fsys fs.FS) (int, error) {
	pluginsDir := filepath.Join(o.Adapter.GlobalConfigDir(o.HomeDir), "plugins")
	count := 0
	err := fs.WalkDir(fsys, "opencode/plugins", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if o.FailAfter > 0 && count >= o.FailAfter {
			return fmt.Errorf("injected failure after %d plugins", o.FailAfter)
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		rel := path[len("opencode/plugins/"):]
		target := filepath.Join(pluginsDir, rel)
		if o.DryRun {
			count++
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := o.tracker.write(target, data, 0644); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}
func (o *OverlayStep) deployPersona(ctx context.Context, fsys fs.FS) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if !o.Adapter.SupportsSystemPrompt() {
		return nil
	}
	promptFile := o.Adapter.SystemPromptFile(o.HomeDir)
	if promptFile == "" {
		return nil
	}
	data, err := fs.ReadFile(fsys, "biggz/biggz-persona.md")
	if err != nil {
		return nil
	}
	content := extractMarkerContent(string(data), "biggz:persona")
	var existing []byte
	if _, err := os.Stat(promptFile); err == nil {
		existing, _ = os.ReadFile(promptFile)
	}
	updated := injectByMarker(string(existing), content, "biggz:persona")
	if o.Adapter.ID() == "pi" {
		if orchData, err := fs.ReadFile(fsys, "biggz/biggz-orchestrator.md"); err == nil {
			updated = injectByMarker(updated, string(orchData), "biggz:orchestrator")
		}
		if webData, err := fs.ReadFile(fsys, "biggz/web-tools.md"); err == nil {
			updated = injectByMarker(updated, string(webData), "biggz:web-tools")
		}
	}
	if o.DryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(promptFile), 0755); err != nil {
		return err
	}
	return o.tracker.write(promptFile, []byte(updated), 0644)
}
func (o *OverlayStep) deployBigMemProtocol(ctx context.Context, fsys fs.FS) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if !o.Adapter.SupportsSystemPrompt() {
		return nil
	}
	promptFile := o.Adapter.SystemPromptFile(o.HomeDir)
	if promptFile == "" {
		return nil
	}
	data, err := fs.ReadFile(fsys, "biggz/bigmem-protocol.md")
	if err != nil {
		return nil
	}
	content := extractMarkerContent(string(data), "biggz:bigmem-protocol")
	var existing []byte
	if _, err := os.Stat(promptFile); err == nil {
		existing, _ = os.ReadFile(promptFile)
	}
	updated := injectByMarker(string(existing), content, "biggz:bigmem-protocol")
	if o.DryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(promptFile), 0755); err != nil {
		return err
	}
	return o.tracker.write(promptFile, []byte(updated), 0644)
}
func (o *OverlayStep) Rollback(ctx context.Context) error {
	_ = ctx
	return o.tracker.rollback()
}
