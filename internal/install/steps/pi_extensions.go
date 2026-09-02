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

// PiExtensionsStep deploys pi-native agents and extensions.
type PiExtensionsStep struct {
	HomeDir string
	Adapter plugin.AgentAdapter
	DryRun  bool
	FS      fs.FS
	Deployed int
	tracker *tracker
	PrepareErr error
	FailAfter int
}

func NewPiExtensionsStep(homeDir string, adapter plugin.AgentAdapter, dryRun bool) *PiExtensionsStep {
	return &PiExtensionsStep{HomeDir: homeDir, Adapter: adapter, DryRun: dryRun, FS: assets.FS, tracker: newTracker()}
}
func (p *PiExtensionsStep) Name() string { return "pi-extensions" }
func (p *PiExtensionsStep) Prepare(ctx context.Context) error {
	if p.PrepareErr != nil {
		return p.PrepareErr
	}
	if p.Adapter == nil {
		return fmt.Errorf("adapter nil")
	}
	if p.HomeDir == "" {
		return fmt.Errorf("homeDir empty")
	}
	// Only pi needs this step; others skip validation.
	if p.Adapter.ID() != "pi" && !strings.Contains(p.Adapter.SkillsDir(p.HomeDir), ".pi") {
		return nil
	}
	fsys := p.FS
	if fsys == nil {
		fsys = assets.FS
	}
	// Validate pi agent assets exist.
	_, err := fs.ReadFile(fsys, "pi/biggz-thinking-wrap.js")
	if err != nil {
		// not fatal for prepare, just ensure FS readable
	}
	_ = ctx
	return nil
}
func (p *PiExtensionsStep) Apply(ctx context.Context, ch pipeline.ProgressChan) error {
	p.tracker.reset()
	p.Deployed = 0
	// Skip for non-pi.
	if p.Adapter.ID() != "pi" && !strings.Contains(p.Adapter.SkillsDir(p.HomeDir), ".pi") {
		if ch != nil {
			select {
			case ch <- pipeline.ProgressEvent{Step: p.Name(), Percent: 100, Message: "skip non-pi"}:
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		return nil
	}
	fsys := p.FS
	if fsys == nil {
		fsys = assets.FS
	}
	// Deploy pi subagents.
	n, err := p.deploySubAgents(ctx, fsys)
	if err != nil {
		return err
	}
	p.Deployed += n
	if ch != nil {
		select {
		case ch <- pipeline.ProgressEvent{Step: p.Name(), Percent: 30, Message: fmt.Sprintf("subagents %d", n)}:
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	// Deploy pi extensions list.
	extensions := []struct{ asset, target string }{
		{"pi/biggz-thinking-wrap.js", "biggz-thinking-wrap.js"},
		{"pi/biggz-memory-chrome.js", "biggz-memory-chrome.js"},
		{"pi/biggz-tool-interception.js", "biggz-tool-interception.js"},
		{"pi/biggz-extension-api.js", "biggz-extension-api.js"},
		{"pi/biggz-last-model.js", "biggz-last-model.js"},
		{"pi/biggz-synthesis-gate.js", "biggz-synthesis-gate.js"},
		{"pi/biggz-wait-pretty.js", "biggz-wait-pretty.js"},
		{"pi/biggz-footer.js", "biggz-footer.js"},
		{"pi/biggz-tool-pills.js", "biggz-tool-pills.js"},
		{"pi/biggz-web-search.js", "biggz-web-search.js"},
		{"pi/biggz-question-mouse.js", "biggz-question-mouse.js"},
	}
	extCount := 0
	for i, e := range extensions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if p.FailAfter > 0 && extCount >= p.FailAfter {
			return fmt.Errorf("injected failure after %d extensions", p.FailAfter)
		}
		data, err := fs.ReadFile(fsys, e.asset)
		if err != nil {
			continue
		}
		extDir := piExtensionsDir(p.HomeDir)
		target := filepath.Join(extDir, e.target)
		if p.DryRun {
			extCount++
			p.Deployed++
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := p.tracker.write(target, data, 0644); err != nil {
			return err
		}
		extCount++
		p.Deployed++
		if ch != nil && i%3 == 0 {
			select {
			case ch <- pipeline.ProgressEvent{Step: p.Name(), Percent: 30 + i*5, Message: e.target}:
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
	}
	// Deploy themes.
	if !p.DryRun {
		if err := p.deployThemes(ctx, fsys); err != nil {
			return err
		}
	}
	// Deploy pi subagent config.
	if err := p.deploySubAgentConfig(ctx, fsys); err != nil {
		return err
	}
	if ch != nil {
		select {
		case ch <- pipeline.ProgressEvent{Step: p.Name(), Percent: 100, Message: "pi done"}:
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	// Self-heal legacy wrapper.
	if !p.DryRun {
		_ = os.Remove(filepath.Join(piExtensionsDir(p.HomeDir), "biggz-pi-pretty.js"))
	}
	return nil
}
func (p *PiExtensionsStep) deploySubAgents(ctx context.Context, fsys fs.FS) (int, error) {
	agentsDir := piAgentsDir(p.HomeDir)
	count := 0
	// Count general/explore first.
	for _, fallback := range []struct{ name, desc, prompt string }{
		{"general", "General unstructured queries", "prompts/general.md"},
		{"explore", "Freeform exploration", "prompts/explore.md"},
	} {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}
		if p.DryRun {
			count++
			continue
		}
		var body string
		if data, err := fs.ReadFile(fsys, fallback.prompt); err == nil {
			body = strings.TrimSpace(string(data))
		}
		if body == "" {
			body = fallback.desc
		}
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			return count, err
		}
		content := fmt.Sprintf("---\nname: %s\ndescription: \"%s\"\ntools:\n  - read\n  - edit\n  - bash\n  - write\n---\n\n%s\n", fallback.name, strings.ReplaceAll(fallback.desc, `"`, `\"`), body)
		if fsys == assets.FS {
			if pdata, err := fs.ReadFile(assets.FS, "biggz/bigmem-protocol.md"); err == nil {
				raw := string(pdata)
				proto := extractMarkerContent(raw, "biggz:bigmem-protocol")
				content += "\n<!-- biggz:bigmem-protocol -->\n" + strings.TrimSpace(proto) + "\n<!-- /biggz:bigmem-protocol -->\n"
			}
		}
		target := filepath.Join(agentsDir, fallback.name+".md")
		if err := p.tracker.write(target, []byte(content), 0644); err != nil {
			return count, err
		}
		count++
	}
	err := fs.WalkDir(fsys, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) != "SKILL.md" {
			return nil
		}
		dir := filepath.Dir(strings.TrimPrefix(path, "skills/"))
		if !strings.HasPrefix(dir, "sdd-") {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if p.FailAfter > 0 && count >= p.FailAfter {
			return fmt.Errorf("injected failure after %d subagents", p.FailAfter)
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		name, desc, body, _ := parseFrontmatter(string(data))
		if name == "" {
			name = dir
		}
		if desc == "" {
			desc = name + " SDD phase"
		}
		if strings.TrimSpace(body) == "" {
			body = "See ~/.biggz/skills/" + dir + "/SKILL.md"
		}
		if p.DryRun {
			count++
			return nil
		}
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			return err
		}
		tools := []string{"read", "edit", "bash", "write", "ask_user_question"}
		if name == "sdd-explore" || name == "sdd-research" {
			tools = []string{"read", "grep", "find", "ls", "ask_user_question"}
		}
		if name == "sdd-research" {
			tools = append(tools, "web_search", "web_fetch")
		}
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("name: %s\n", name))
		sb.WriteString(fmt.Sprintf("description: \"%s\"\n", strings.ReplaceAll(desc, `"`, `\"`)))
		sb.WriteString("tools:\n")
		for _, t := range tools {
			sb.WriteString(fmt.Sprintf("  - %s\n", t))
		}
		sb.WriteString("---\n\n")
		sb.WriteString(strings.TrimSpace(body))
		sb.WriteString("\n")
		if pdata, err := fs.ReadFile(assets.FS, "biggz/bigmem-protocol.md"); err == nil {
			raw := string(pdata)
			proto := extractMarkerContent(raw, "biggz:bigmem-protocol")
			sb.WriteString("\n<!-- biggz:bigmem-protocol -->\n" + strings.TrimSpace(proto) + "\n<!-- /biggz:bigmem-protocol -->\n")
		} else if fsys != assets.FS {
			if pdata, err := fs.ReadFile(fsys, "biggz/bigmem-protocol.md"); err == nil {
				raw := string(pdata)
				proto := extractMarkerContent(raw, "biggz:bigmem-protocol")
				sb.WriteString("\n<!-- biggz:bigmem-protocol -->\n" + strings.TrimSpace(proto) + "\n<!-- /biggz:bigmem-protocol -->\n")
			}
		}
		target := filepath.Join(agentsDir, name+".md")
		if err := p.tracker.write(target, []byte(sb.String()), 0644); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}
func (p *PiExtensionsStep) deployThemes(ctx context.Context, fsys fs.FS) error {
	themesDir := filepath.Join(piAgentsDir(p.HomeDir), "..", "themes")
	themesDir = filepath.Clean(themesDir)
	return fs.WalkDir(fsys, "pi/themes", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if strings.Contains(err.Error(), "file does not exist") {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		target := filepath.Join(themesDir, filepath.Base(path))
		if err := os.MkdirAll(themesDir, 0755); err != nil {
			return err
		}
		return p.tracker.write(target, data, 0644)
	})
}
func (p *PiExtensionsStep) deploySubAgentConfig(ctx context.Context, fsys fs.FS) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	var assetData []byte
	var err error
	if fsys != nil {
		assetData, err = fs.ReadFile(fsys, "pi/subagent-config.json")
	}
	if err != nil || len(assetData) == 0 {
		assetData, err = fs.ReadFile(assets.FS, "pi/subagent-config.json")
		if err != nil {
			return nil
		}
	}
	if p.DryRun {
		return nil
	}
	configDir := filepath.Join(piExtensionsDir(p.HomeDir), "subagent")
	target := filepath.Join(configDir, "config.json")
	var existing []byte
	if _, err := os.Stat(target); err == nil {
		existing, _ = os.ReadFile(target)
	}
	var merged []byte
	if len(existing) > 0 {
		merged, err = mergeJSONCWrapper(existing, assetData)
		if err != nil {
			merged, _ = mergeJSONCWrapper([]byte("{}"), assetData)
		}
	} else {
		merged, _ = mergeJSONCWrapper([]byte("{}"), assetData)
	}
	if len(merged) > 0 && merged[len(merged)-1] != '\n' {
		merged = append(merged, '\n')
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	return p.tracker.write(target, merged, 0644)
}
func (p *PiExtensionsStep) Rollback(ctx context.Context) error {
	_ = ctx
	return p.tracker.rollback()
}

