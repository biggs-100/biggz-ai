package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/pipeline"
	"github.com/biggs-100/biggz-ai/plugin"
)

// PiExtensionsStep deploys pi-native agents and extensions.
type PiExtensionsStep struct {
	HomeDir    string
	Adapter    plugin.AgentAdapter
	DryRun     bool
	FS         fs.FS
	Deployed   int
	tracker    *tracker
	PrepareErr error
	FailAfter  int
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
	if err := validatePiExtensionsFactory(fsys); err != nil {
		return err
	}
	if err := validatePiExtensionsLayout(fsys); err != nil {
		return err
	}
	_ = ctx
	return nil
}

// piShellSubdir is the subdirectory under the pi agent's extensions/ directory
// that holds shell helper modules. A helper cannot sit flat: pi loads every
// direct extensions/*.js|*.ts as an extension and crashes on one that does not
// export a factory. pi loads a subdirectory only when it carries an
// index.js/index.ts or a package.json with a `pi` field, so helpers parked here
// stay importable by the flat factories while remaining invisible to the loader.
const piShellSubdir = "biggz-shell"

// piExtension is one entry of the pi extensions deploy list.
//
//   - asset is the path inside the embedded assets FS.
//   - target is the path relative to the pi agent's extensions/ directory.
//   - factory means "pi loads this file as an extension, so it must export
//     `export default function(pi)`".
//
// The role is recorded rather than inferred from the file extension because this
// repository ships TypeScript factories (ask-user-choice.ts, codegraph-tools.ts,
// skill-registry.ts) that pi loads as extensions. The old extension-suffix
// inference treated every TS asset as a non-factory and silently skipped
// validating those three deployed extensions.
type piExtension struct {
	asset   string
	target  string
	factory bool
}

// piExtensionsDeployList returns the canonical deploy list for pi extensions.
// Two test mirrors repeat the FACTORY subset: internal/assets/pi/
// biggz-pi-extensions-factory.test.mjs and the mirror list in
// pi_extensions_guard_test.go. The Go cross-check compares the factory count
// only and the JS mirror imports no canonical list, so a renamed or dropped
// factory entry is caught by neither, and a helper rename or drop is caught by
// nothing: the three lists agree today, but nothing keeps them agreeing. Task
// 2.12 tracks the deploy-manifest parity guard.
// biggz-synthesis-gate.js wrapper is excluded: native Go synthesis gate
// (internal/sdd/synthesis_gate.go) is the only enforcement path. JS source
// stays on disk until the Phase-2 stability gate + one-release soak allows
// deletion.
// biggz-memory-chrome.js is DEPLOYED: it is the pretty layer that collapses
// native /mcp memory results to one line (no gate — always active unless
// BIGGZ_PRETTY=0).
// biggz-subagent-runtime.js is DEPLOYED: it owns pi delegation after the
// j0k3r cutover (agent discovery, one RPC child per task, background delivery,
// widget, `subagent*` tools). biggz-wait-pretty.js is retired with the fork —
// the runtime's `subagent_wait` carries the headline.
//
// Every entry in this list is currently a factory, including the three TS
// files: pi loads every direct extensions/*.js|*.ts as an extension. Helpers go
// under piShellSubdir/ with factory:false, after the factory block.
func piExtensionsDeployList() []piExtension {
	return []piExtension{
		{asset: "pi/biggz-thinking-wrap.js", target: "biggz-thinking-wrap.js", factory: true},
		{asset: "pi/biggz-memory-chrome.js", target: "biggz-memory-chrome.js", factory: true},
		{asset: "pi/biggz-tool-interception.js", target: "biggz-tool-interception.js", factory: true},
		{asset: "pi/biggz-extension-api.js", target: "biggz-extension-api.js", factory: true},
		{asset: "pi/biggz-session-guard.js", target: "biggz-session-guard.js", factory: true},
		{asset: "pi/biggz-ask-guard.js", target: "biggz-ask-guard.js", factory: true},
		{asset: "pi/biggz-last-model.js", target: "biggz-last-model.js", factory: true},
		{asset: "pi/biggz-subagent-runtime.js", target: "biggz-subagent-runtime.js", factory: true},
		{asset: "pi/biggz-shell.ts", target: "biggz-shell.ts", factory: true},
		{asset: "pi/biggz-tool-pills.js", target: "biggz-tool-pills.js", factory: true},
		{asset: "pi/biggz-web-search.js", target: "biggz-web-search.js", factory: true},
		{asset: "pi/biggz-question-mouse.js", target: "biggz-question-mouse.js", factory: true},
		{asset: "pi/biggz-quiet-tools.js", target: "biggz-quiet-tools.js", factory: true},
		{asset: "pi/ask-user-choice.ts", target: "ask-user-choice.ts", factory: true},
		{asset: "pi/codegraph-tools.ts", target: "codegraph-tools.ts", factory: true},
		{asset: "pi/skill-registry.ts", target: "skill-registry.ts", factory: true},
		// Shell helpers: pure drawing primitives imported by the shell factories.
		// pi must never load them as extensions, so they live in piShellSubdir/.
		{asset: "pi/biggz-shell/shell-gauge.ts", target: piShellSubdir + "/shell-gauge.ts", factory: false},
		{asset: "pi/biggz-shell/shell-hover.ts", target: piShellSubdir + "/shell-hover.ts", factory: false},
		{asset: "pi/biggz-shell/terminal-theme.ts", target: piShellSubdir + "/terminal-theme.ts", factory: false},
		{asset: "pi/biggz-shell/shell-card.ts", target: piShellSubdir + "/shell-card.ts", factory: false},
		{asset: "pi/biggz-shell/shell-changes.ts", target: piShellSubdir + "/shell-changes.ts", factory: false},
		{asset: "pi/biggz-shell/shell-usage.ts", target: piShellSubdir + "/shell-usage.ts", factory: false},
		{asset: "pi/biggz-shell/shell-usage-view.ts", target: piShellSubdir + "/shell-usage-view.ts", factory: false},
		{asset: "pi/biggz-shell/shell-todo.ts", target: piShellSubdir + "/shell-todo.ts", factory: false},
		{asset: "pi/biggz-shell/shell-prompt.ts", target: piShellSubdir + "/shell-prompt.ts", factory: false},
		{asset: "pi/biggz-shell/shell-sidebar.ts", target: piShellSubdir + "/shell-sidebar.ts", factory: false},
		{asset: "pi/biggz-shell/shell-sidebar-layout.ts", target: piShellSubdir + "/shell-sidebar-layout.ts", factory: false},
		{asset: "pi/biggz-shell/review-sidebar-state.ts", target: piShellSubdir + "/review-sidebar-state.ts", factory: false},
		{asset: "pi/biggz-shell/agent-home.ts", target: piShellSubdir + "/agent-home.ts", factory: false},
		{asset: "pi/biggz-shell/animation-policy.ts", target: piShellSubdir + "/animation-policy.ts", factory: false},
		{asset: "pi/biggz-shell/visual-customization-policy.ts", target: piShellSubdir + "/visual-customization-policy.ts", factory: false},
		{asset: "pi/biggz-shell/theme-customization.ts", target: piShellSubdir + "/theme-customization.ts", factory: false},
		{asset: "pi/biggz-shell/shell-bar.ts", target: piShellSubdir + "/shell-bar.ts", factory: false},
		{asset: "pi/biggz-shell/shell-sidebar-banner.ts", target: piShellSubdir + "/shell-sidebar-banner.ts", factory: false},
		{asset: "pi/biggz-shell/native-pointer-region.ts", target: piShellSubdir + "/native-pointer-region.ts", factory: false},
	}
}

// retiredPiExtensionTargets lists pi extension file names that are no longer in
// piExtensionsDeployList but may still exist in an already-installed home, so
// Apply removes them. Absent files are a silent no-op, which is the existing
// self-heal contract (os.Remove error ignored).
//
// biggz-synthesis-gate.js: the native Go synthesis gate
// (internal/sdd/synthesis_gate.go) is the only enforcement path, so a stale copy
// would keep enforcing the retired duplicate gate.
// biggz-wait-pretty.js: retired with the j0k3r cutover; a stale copy keeps
// wrapping hooks that no longer exist.
// biggz-footer.js: retired in unit 2.4, when biggz-shell.ts took over the footer
// slot. The asset stays in the repository as the rollback reference, but a stale
// installed copy would register a second footer.
// biggz-memory-chrome.js is NOT retired — it is deployed (see
// piExtensionsDeployList).
var retiredPiExtensionTargets = []string{"biggz-synthesis-gate.js", "biggz-wait-pretty.js", "biggz-footer.js"}

// validatePiExtensionsFactory asserts the factory role recorded on every deploy
// entry, regardless of file extension. For a factory the asset must contain
// `export default function` (pi otherwise crashes with "Extension does not
// export a valid factory function"). For a helper (factory false) the asset must
// NOT contain `export default function`: a helper that looks like an extension
// is one that will eventually be moved flat by mistake and crash pi.
//
// The check follows the recorded role, not the suffix: the repository ships TS
// factories, and the previous suffix rule silently skipped ask-user-choice.ts,
// codegraph-tools.ts and skill-registry.ts.
//
// Runtime width smoke (future): the deployed single-line surface is the shell
// bar, and it must never render wider than the terminal. Currently enforced via
// the JS gates internal/assets/pi/biggz-shell/shell-bar-themes.test.mjs (791
// width assertions across the seven shipped themes: it renders the bar, header,
// rule and sidebar card at five widths and fails on any role a theme does not
// define) and internal/assets/pi/biggz-shell.test.mjs (the registered footer
// paints exactly one line at widths 20..200). Parity with
// internal/tui/sanitize.go (go-runewidth/StringWidth + TruncateToWidth) per
// tui-sanitize spec. Not duplicating in Go to avoid over-engineering; installer
// already validates factory export. This comment is the width-gate anchor for a
// future Go runtime check if needed.
func validatePiExtensionsFactory(fsys fs.FS) error {
	if fsys == nil {
		fsys = assets.FS
	}
	for _, e := range piExtensionsDeployList() {
		data, err := fs.ReadFile(fsys, e.asset)
		if err != nil {
			// Missing file is not a factory error — test stubs may provide
			// partial FS. Real embedded assets always exist; missing there
			// would be caught by deploy loop's silent continue, but we prefer
			// to surface it as validation when possible.
			continue
		}
		content := string(data)
		if !e.factory {
			if strings.Contains(content, "export default function") {
				return fmt.Errorf("pi shell helper %s looks like an extension: it contains 'export default function', but %s is deployed under %s/ where pi must not load it as an extension — drop the factory export or move the module flat", e.asset, e.target, piShellSubdir)
			}
			continue
		}
		if !strings.Contains(content, "export default") {
			return fmt.Errorf("pi extension %s missing factory: must contain 'export default' (pi requires valid factory) — %s will crash pi with 'Extension does not export a valid factory function'", e.asset, e.target)
		}
		if !strings.Contains(content, "export default function") {
			return fmt.Errorf("pi extension %s missing factory function: must contain 'export default function' (pi loader expects factory function) — %s", e.asset, e.target)
		}
	}
	return nil
}

// validatePiExtensionsLayout asserts the hybrid deploy shape: factories flat in
// the extensions directory, helpers sealed in a subdirectory pi never loads as
// an extension. It composes the pure target rules with the embedded-manifest
// rule so a broken shape fails fast, before Apply writes anything.
func validatePiExtensionsLayout(fsys fs.FS) error {
	if err := validatePiExtensionsTargets(piExtensionsDeployList()); err != nil {
		return err
	}
	return validatePiShellSubdirManifest(fsys)
}

// validatePiExtensionsTargets checks the target paths of a deploy list without
// touching a filesystem, so the shape rules are testable with synthetic lists:
// a helper must live under piShellSubdir/, a factory must be a direct child of
// the extensions directory, no helper may be an index file (which would make pi
// load the subdirectory as an extension), and two entries may not share a
// target.
func validatePiExtensionsTargets(list []piExtension) error {
	seen := make(map[string]struct{}, len(list))
	for _, e := range list {
		if _, dup := seen[e.target]; dup {
			return fmt.Errorf("duplicate pi extension target %q: two deploy entries would overwrite each other", e.target)
		}
		seen[e.target] = struct{}{}
		if e.factory {
			if strings.ContainsAny(e.target, "/\\") {
				return fmt.Errorf("pi extension factory %q must be a direct child of the extensions directory: pi loads only direct extensions/*.js|*.ts entries as extensions, so a factory may not be a nested target", e.target)
			}
			continue
		}
		if !strings.HasPrefix(e.target, piShellSubdir+"/") {
			return fmt.Errorf("pi shell helper %q must target a path under %s/: a helper parked flat is loaded by pi as an extension and crashes it", e.target, piShellSubdir)
		}
		switch path.Base(e.target) {
		case "index.js", "index.ts", "index.mjs":
			return fmt.Errorf("pi shell helper %q may not be an index file: pi loads %s/ as an extension when it carries index.js/index.ts, which is exactly what this design depends on not happening", e.target, piShellSubdir)
		}
	}
	return nil
}

// validatePiShellSubdirManifest rejects a pi/biggz-shell/package.json that
// declares a `pi` field: such a manifest makes pi load the subdirectory as an
// extension. An absent manifest is fine, and any other manifest content is out
// of scope here.
func validatePiShellSubdirManifest(fsys fs.FS) error {
	if fsys == nil {
		fsys = assets.FS
	}
	manifest := "pi/" + piShellSubdir + "/package.json"
	data, err := fs.ReadFile(fsys, manifest)
	if err != nil {
		return nil
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("pi shell subdir manifest %s is not valid JSON: %w", manifest, err)
	}
	if _, ok := doc["pi"]; ok {
		return fmt.Errorf("pi shell subdir manifest %s declares a `pi` field: pi would load %s/ as an extension, which this design depends on not happening — remove the field", manifest, piShellSubdir)
	}
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
	// Fail fast if any deployed extension lacks a valid factory, or if the
	// hybrid layout is broken — prevents deploying a shape that would crash
	// pi on startup.
	if err := validatePiExtensionsFactory(fsys); err != nil {
		return err
	}
	if err := validatePiExtensionsLayout(fsys); err != nil {
		return err
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
	extensions := piExtensionsDeployList()
	// NOTE: gentle-ai.ts, quiet-tools.ts and sdd-init.ts were removed from
	// internal/assets/pi (2026-09-04): they imported from ../lib/*
	// (sdd-preflight, gentle-ai-binary, review-*, terminal-theme, ...), a
	// tree that was never ported, and they bound to the gentle-ai binary
	// instead of biggz. Deploying them crashed pi on startup ("Cannot find
	// module '../lib/sdd-preflight.ts'"). Their features are covered
	// natively by the Go synthesis gate (internal/sdd/synthesis_gate.go),
	// biggz-session-guard.js, biggz-tool-pills.js,
	// skill-registry.ts and the biggz review/SDD CLI commands.
	// (2026-09-08 pi-wrapper-removal: biggz-synthesis-gate.js no longer
	// deployed either — excluded from piExtensionsDeployList with stale
	// self-heal; native /mcp via pi-mcp-adapter@^2 is the only memory path.)
	if _, err := p.deployExtensionFiles(ctx, fsys, extensions, ch); err != nil {
		return err
	}
	// Deploy themes.
	if !p.DryRun {
		if err := p.deployThemes(ctx, fsys); err != nil {
			return err
		}
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
		// Self-heal: remove unportable extensions deployed before they were
		// dropped from the deploy list (see NOTE above). Left in place they
		// crash pi on startup with "Cannot find module '../lib/...'".
		extDir := piExtensionsDir(p.HomeDir)
		for _, stale := range []string{"gentle-ai.ts", "quiet-tools.ts", "sdd-init.ts", "startup-banner.ts"} {
			_ = os.Remove(filepath.Join(extDir, stale))
		}
		// Self-heal retired wrappers: remove every target in
		// retiredPiExtensionTargets (biggz-synthesis-gate.js,
		// biggz-wait-pretty.js, biggz-footer.js). Each entry's reason lives on
		// the variable. Absent files are a silent no-op (os.Remove error
		// ignored), and biggz-memory-chrome.js is NOT stale — it is deployed
		// (see piExtensionsDeployList).
		for _, stale := range retiredPiExtensionTargets {
			_ = os.Remove(filepath.Join(extDir, stale))
		}
		_ = os.RemoveAll(filepath.Join(extDir, "_disabled_broken_backup"))
	}
	return nil
}

// deployExtensionFiles writes every entry of list into the pi agent's
// extensions/ directory, creating the target's parent directory so nested
// helpers (targets under piShellSubdir) land in their subdirectory. It returns
// the number of entries it wrote or would write (DryRun counts too); an entry
// whose asset is unreadable is skipped silently and not counted, preserving the
// original loop's continue. Extracted from Apply so the hybrid shape is
// testable without a full install run.
func (p *PiExtensionsStep) deployExtensionFiles(ctx context.Context, fsys fs.FS, list []piExtension, ch pipeline.ProgressChan) (int, error) {
	extDir := piExtensionsDir(p.HomeDir)
	extCount := 0
	for i, e := range list {
		select {
		case <-ctx.Done():
			return extCount, ctx.Err()
		default:
		}
		if p.FailAfter > 0 && extCount >= p.FailAfter {
			return extCount, fmt.Errorf("injected failure after %d extensions", p.FailAfter)
		}
		data, err := fs.ReadFile(fsys, e.asset)
		if err != nil {
			continue
		}
		target := filepath.Join(extDir, e.target)
		if p.DryRun {
			extCount++
			p.Deployed++
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return extCount, err
		}
		if err := p.tracker.write(target, data, 0644); err != nil {
			return extCount, err
		}
		extCount++
		p.Deployed++
		if ch != nil && i%3 == 0 {
			select {
			case ch <- pipeline.ProgressEvent{Step: p.Name(), Percent: 30 + i*5, Message: e.target}:
			case <-ctx.Done():
				return extCount, ctx.Err()
			default:
			}
		}
	}
	return extCount, nil
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
		// Children must not ship an ask tool: asking the human is the orchestrator's job, which
		// owns the checkpoint surface and the synthesis gate. A child dialog is answered by the
		// runtime (background) or relayed as a named subagent question (foreground), and a child
		// that needs a decision returns status blocked with its reason.
		tools := []string{"read", "edit", "bash", "write"}
		if name == "sdd-explore" || name == "sdd-research" {
			tools = []string{"read", "grep", "find", "ls"}
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
func (p *PiExtensionsStep) Rollback(ctx context.Context) error {
	_ = ctx
	return p.tracker.rollback()
}
