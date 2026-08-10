package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/internal/opencode"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	"github.com/biggs-100/biggz-ai/model"
	tea "github.com/charmbracelet/bubbletea"
)

type mpView int

const (
	mpPhaseList mpView = iota
	mpProvider
	mpModel
	mpEffort
)

// mpMaxVisible is the maximum number of rows rendered in any picker list.
const mpMaxVisible = 12

// mpAgentRows is the flat, grouped list of configurable agents:
// orchestrator, SDD phases, judgment-day agents, and review agents.
var mpAgentRows = buildMpAgentRows()

func buildMpAgentRows() []string {
	rows := []string{opencode.OrchestratorAgent}
	rows = append(rows, "--- SDD phases ---")
	rows = append(rows, opencode.SDDPhases()...)
	rows = append(rows, "--- Judgment Day ---")
	rows = append(rows, opencode.JDPhases()...)
	rows = append(rows, "--- Review agents ---")
	rows = append(rows, opencode.ReviewPhases()...)
	return rows
}

// mpProviderEntry is a provider row in the provider picker.
type mpProviderEntry struct {
	ID         string
	Name       string
	ModelCount int
}

// ModelPickerScreen lets the user assign models to every configurable agent,
// driven by the OpenCode model cache and the biggz model-variants plugin
// cache. Selections are persisted straight to opencode.json (biggz direct
// merge style) with JSONC-safe helpers.
type ModelPickerScreen struct {
	view   mpView
	cursor int

	cachePath    string
	variantsPath string
	settingsPath string

	providers        map[string]opencode.Provider
	providerEntries  []mpProviderEntry
	selectedProvider string
	models           []opencode.Model
	selectedAgent    string
	pending          model.ModelAssignment
	effortLevels     []string

	assignments map[string]model.ModelAssignment
	warning     string
	status      string
	err         string
}

// NewModelPickerScreen creates the model picker with the default cache,
// variants, and settings paths.
func NewModelPickerScreen() ModelPickerScreen {
	return NewModelPickerScreenWithPaths(
		opencode.DefaultCachePath(),
		opencode.DefaultVariantsCachePath(),
		opencode.DefaultSettingsPath(),
	)
}

// NewModelPickerScreenWithPaths creates the model picker with explicit paths
// (used by tests so the real home directory is never touched).
func NewModelPickerScreenWithPaths(cachePath, variantsPath, settingsPath string) ModelPickerScreen {
	providers, _ := opencode.LoadModelsOrEmpty(cachePath)
	opencode.EnrichWithVariants(providers, variantsPath)
	assignments, _ := opencode.ReadCurrentModelAssignments(settingsPath)
	sanitizeStaleEfforts(assignments, providers)

	m := ModelPickerScreen{
		view:         mpPhaseList,
		providers:    providers,
		cachePath:    cachePath,
		variantsPath: variantsPath,
		settingsPath: settingsPath,
		assignments:  assignments,
	}
	if len(providers) == 0 {
		m.warning = "Model cache not found — run 'opencode' once to populate ~/.cache/opencode/models.json."
	}
	m.providerEntries = buildProviderEntries(providers)
	return m
}

// sanitizeStaleEfforts clears an assignment's effort when the model cache knows
// the model but the stored effort is not among its current variant levels.
// Models unknown to the cache are left untouched.
func sanitizeStaleEfforts(assignments map[string]model.ModelAssignment, providers map[string]opencode.Provider) {
	for agent, a := range assignments {
		p, ok := providers[a.ProviderID]
		if !ok {
			continue
		}
		cached, ok := p.Models[a.ModelID]
		if !ok || len(cached.Variants) == 0 || a.Effort == "" {
			continue
		}
		if !containsString(cached.Variants, a.Effort) {
			a.Effort = ""
			assignments[agent] = a
		}
	}
}

// buildProviderEntries returns providers that have at least one tool_call
// capable model, sorted by display name.
func buildProviderEntries(providers map[string]opencode.Provider) []mpProviderEntry {
	entries := make([]mpProviderEntry, 0, len(providers))
	for id, p := range providers {
		count := len(opencode.FilterModelsForSDD(p))
		if count == 0 {
			continue
		}
		name := p.Name
		if name == "" {
			name = id
		}
		entries = append(entries, mpProviderEntry{ID: id, Name: name, ModelCount: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

func (m ModelPickerScreen) Init() tea.Cmd { return nil }

func (m ModelPickerScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "enter":
		return m.handleEnter()
	case "esc":
		return m.handleEsc()
	}
	return m, nil
}

// moveCursor moves the list cursor within the current mode. Phase-list
// navigation skips separator rows.
func (m *ModelPickerScreen) moveCursor(delta int) {
	count := m.listLen()
	if count == 0 {
		return
	}
	for {
		next := m.cursor + delta
		if next < 0 || next >= count {
			return
		}
		m.cursor = next
		if m.view == mpPhaseList && isMpSeparatorRow(mpAgentRows[next]) {
			continue
		}
		return
	}
}

func (m *ModelPickerScreen) listLen() int {
	switch m.view {
	case mpPhaseList:
		return len(mpAgentRows)
	case mpProvider:
		return len(m.providerEntries)
	case mpModel:
		return len(m.models)
	case mpEffort:
		return len(mpEffortOptions(m.effortLevels))
	}
	return 0
}

func (m *ModelPickerScreen) handleEnter() (tea.Model, tea.Cmd) {
	switch m.view {
	case mpPhaseList:
		row := mpAgentRows[m.cursor]
		if isMpSeparatorRow(row) {
			return *m, nil
		}
		if len(m.providerEntries) == 0 {
			return *m, nil
		}
		m.selectedAgent = row
		m.view = mpProvider
		m.cursor = 0

	case mpProvider:
		if m.cursor >= len(m.providerEntries) {
			return *m, nil
		}
		entry := m.providerEntries[m.cursor]
		m.selectedProvider = entry.ID
		m.models = opencode.FilterModelsForSDD(m.providers[entry.ID])
		m.view = mpModel
		m.cursor = 0

	case mpModel:
		if len(m.models) == 0 || m.cursor >= len(m.models) {
			return *m, nil
		}
		selected := m.models[m.cursor]
		assignment := model.ModelAssignment{
			ProviderID: m.selectedProvider,
			ModelID:    selected.ID,
		}
		if levels := selected.EffortLevels(); len(levels) > 0 {
			// Model has variants: ask for an effort level first.
			m.pending = assignment
			m.effortLevels = levels
			m.view = mpEffort
			m.cursor = 0
			return *m, nil
		}
		// No variants: apply directly. Preserve the stored effort only for
		// reasoning models whose variant metadata is missing.
		m.finalizeAssignment(assignment, selected.Reasoning)

	case mpEffort:
		opts := mpEffortOptions(m.effortLevels)
		if m.cursor >= len(opts) {
			return *m, nil
		}
		effort := opts[m.cursor]
		if effort == "default" {
			effort = ""
		}
		assignment := m.pending
		assignment.Effort = effort
		m.finalizeAssignment(assignment, false)
	}
	return *m, nil
}

func (m *ModelPickerScreen) handleEsc() (tea.Model, tea.Cmd) {
	switch m.view {
	case mpPhaseList:
		return *m, func() tea.Msg { return NavigateMsg{Screen: 0} }
	case mpProvider:
		m.view = mpPhaseList
	case mpModel:
		m.view = mpProvider
	case mpEffort:
		m.view = mpModel
	}
	m.cursor = 0
	return *m, nil
}

// finalizeAssignment stores the assignment, persists it to opencode.json, and
// returns to the phase list.
func (m *ModelPickerScreen) finalizeAssignment(assignment model.ModelAssignment, preserveEffort bool) {
	existing := m.assignments[m.selectedAgent]
	if preserveEffort && existing.ProviderID == assignment.ProviderID && existing.ModelID == assignment.ModelID {
		assignment.Effort = existing.Effort
	}
	m.assignments[m.selectedAgent] = assignment

	m.view = mpPhaseList
	m.cursor = 0
	m.status = ""
	m.err = ""
	if err := m.persist(); err != nil {
		m.err = err.Error()
		return
	}
	label := assignment.FullID()
	if assignment.Effort != "" {
		label += " [" + assignment.Effort + "]"
	}
	m.status = fmt.Sprintf("Saved %s → %s", m.selectedAgent, label)
}

// persist writes the current assignments into opencode.json: it reads the
// existing file (JSONC-safe), injects model/variant fields into the embedded
// SDD overlay, deep-merges, and writes atomically.
func (m *ModelPickerScreen) persist() error {
	existing, err := os.ReadFile(m.settingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read settings %q: %w", m.settingsPath, err)
		}
		existing = []byte("{}")
	}

	rootModelID := mpRootModel(existing)
	existingKeys := mpExistingAgentKeys(existing)

	overlay, err := assets.FS.ReadFile("opencode/sdd-overlay-multi.json")
	if err != nil {
		return fmt.Errorf("read SDD overlay asset: %w", err)
	}

	injected, err := opencode.InjectModelAssignments(overlay, m.assignments, rootModelID, existingKeys)
	if err != nil {
		return fmt.Errorf("inject model assignments: %w", err)
	}

	merged, err := filemerge.MergeJSONC(existing, injected)
	if err != nil {
		return fmt.Errorf("merge model assignments: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.settingsPath), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}
	if _, err := filemerge.WriteFileAtomic(m.settingsPath, merged, 0o644); err != nil {
		return fmt.Errorf("write settings %q: %w", m.settingsPath, err)
	}
	return nil
}

// mpRootModel reads the top-level "model" field from opencode.json settings
// (JSONC-safe). Returns "" when absent or unparseable.
func mpRootModel(settings []byte) string {
	root, err := filemerge.UnmarshalJSONObject(settings)
	if err != nil {
		return ""
	}
	modelID, _ := root["model"].(string)
	return modelID
}

// mpExistingAgentKeys returns the set of agent keys already present in
// opencode.json settings (JSONC-safe). Empty when absent or unparseable.
func mpExistingAgentKeys(settings []byte) []string {
	root, err := filemerge.UnmarshalJSONObject(settings)
	if err != nil {
		return nil
	}
	agentRaw, ok := root["agent"]
	if !ok {
		return nil
	}
	agentMap, ok := agentRaw.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(agentMap))
	for name := range agentMap {
		keys = append(keys, name)
	}
	return keys
}

// mpEffortOptions returns the effort picker options in display order. The
// first entry ("default") maps to an empty Effort string (provider default).
// Levels literally named "default" are excluded to prevent duplicates.
func mpEffortOptions(levels []string) []string {
	opts := make([]string, 0, len(levels)+1)
	opts = append(opts, "default")
	for _, level := range levels {
		if level != "default" {
			opts = append(opts, level)
		}
	}
	return opts
}

func isMpSeparatorRow(row string) bool {
	return strings.HasPrefix(row, "--- ")
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func (m ModelPickerScreen) View() string {
	var b strings.Builder

	switch m.view {
	case mpPhaseList:
		b.WriteString(styles.Title.Render("Model Assignments"))
		b.WriteString("\n\n")
		if m.warning != "" {
			b.WriteString(styles.WarningBox.Render(m.warning))
			b.WriteString("\n\n")
		}
		if m.err != "" {
			b.WriteString(styles.ErrorBox.Render("Error: " + m.err))
			b.WriteString("\n\n")
		}
		if m.status != "" {
			b.WriteString(styles.SuccessBox.Render(m.status))
			b.WriteString("\n\n")
		}

		start, end := mpWindow(m.cursor, len(mpAgentRows))
		for i := start; i < end; i++ {
			row := mpAgentRows[i]
			if isMpSeparatorRow(row) {
				b.WriteString(styles.StatusInfo.Render("  " + row))
				b.WriteString("\n")
				continue
			}
			label := mpAssignmentLabel(row, m.assignments[row], m.providers)
			cur := "  "
			if i == m.cursor {
				cur = "▸ "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", cur, label))
		}

		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑↓ navigate · ENTER change model · ESC back"))

	case mpProvider:
		b.WriteString(styles.Title.Render("Select Provider"))
		b.WriteString("\n\n")
		if len(m.providerEntries) == 0 {
			b.WriteString(styles.StatusDisabled.Render("No providers with tool-call models found."))
			b.WriteString("\n")
			b.WriteString(styles.StatusInfo.Render("Run 'opencode' once to populate the model cache."))
			b.WriteString("\n\n")
		} else {
			start, end := mpWindow(m.cursor, len(m.providerEntries))
			for i := start; i < end; i++ {
				entry := m.providerEntries[i]
				cur := "  "
				if i == m.cursor {
					cur = "▸ "
				}
				label := fmt.Sprintf("%s (%d models)", entry.Name, entry.ModelCount)
				b.WriteString(fmt.Sprintf("%s%s\n", cur, label))
			}
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑↓ navigate · ENTER select · ESC back"))

	case mpModel:
		b.WriteString(styles.Title.Render("Select Model — " + m.providerName(m.selectedProvider)))
		b.WriteString("\n\n")
		if len(m.models) == 0 {
			b.WriteString(styles.StatusDisabled.Render("No tool-call capable models for this provider."))
			b.WriteString("\n\n")
		} else {
			start, end := mpWindow(m.cursor, len(m.models))
			for i := start; i < end; i++ {
				mod := m.models[i]
				cur := "  "
				if i == m.cursor {
					cur = "▸ "
				}
				label := mod.Name
				if mod.ID != "" && mod.ID != mod.Name {
					label += " (" + mod.ID + ")"
				}
				if mod.Cost > 0 {
					label += fmt.Sprintf("  $%.2f/1M in", mod.Cost)
				}
				b.WriteString(fmt.Sprintf("%s%s\n", cur, label))
			}
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑↓ navigate · ENTER select · ESC back"))

	case mpEffort:
		b.WriteString(styles.Title.Render("Select Reasoning Effort"))
		b.WriteString("\n\n")
		opts := mpEffortOptions(m.effortLevels)
		for i, opt := range opts {
			cur := "  "
			if i == m.cursor {
				cur = "▸ "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", cur, opt))
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑↓ navigate · ENTER select · ESC back"))
	}

	return styles.AppStyle.Render(b.String())
}

// mpAssignmentLabel renders an agent row with its current assignment, e.g.
// "sdd-init  Anthropic / Claude Sonnet 4 [high]" or "(default)".
func mpAssignmentLabel(agent string, a model.ModelAssignment, providers map[string]opencode.Provider) string {
	if a.ProviderID == "" || a.ModelID == "" {
		return fmt.Sprintf("%-20s %s", agent, "(default)")
	}
	provName, modelName := a.ProviderID, a.ModelID
	if p, ok := providers[a.ProviderID]; ok {
		if p.Name != "" {
			provName = p.Name
		}
		if mm, ok := p.Models[a.ModelID]; ok && mm.Name != "" {
			modelName = mm.Name
		}
	}
	label := provName + " / " + modelName
	if a.Effort != "" {
		label += " [" + a.Effort + "]"
	}
	return fmt.Sprintf("%-20s %s", agent, label)
}

func (m ModelPickerScreen) providerName(id string) string {
	if p, ok := m.providers[id]; ok && p.Name != "" {
		return p.Name
	}
	return id
}

// mpWindow returns the visible [start, end) window of a list, centered on the
// cursor so navigation never leaves the selection off-screen.
func mpWindow(cursor, count int) (int, int) {
	if count <= mpMaxVisible {
		return 0, count
	}
	start := cursor - mpMaxVisible/2
	if start < 0 {
		start = 0
	}
	end := start + mpMaxVisible
	if end > count {
		end = count
		start = end - mpMaxVisible
	}
	return start, end
}
