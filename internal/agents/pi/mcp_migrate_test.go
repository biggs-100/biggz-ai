package pi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// adapterMigrationFixture lays out an adapter-era install in a temp home: a Pi
// agent dir (settings.json + mcp.json) plus the OpenCode global config the
// pi-mcp-adapter used to import servers from.
//
// Hermetic by construction: PI_CODING_AGENT_DIR is cleared and every path lives
// under t.TempDir(), so no test can reach a real agent home.
type adapterMigrationFixture struct {
	home        string
	agentDir    string
	opencodeDir string
}

func newAdapterMigrationFixture(t *testing.T) adapterMigrationFixture {
	t.Helper()
	t.Setenv("PI_SUBAGENT_CHILD", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	home := t.TempDir()
	return adapterMigrationFixture{
		home:        home,
		agentDir:    filepath.Join(home, ".pi", "agent"),
		opencodeDir: filepath.Join(home, ".config", "opencode"),
	}
}

func (f adapterMigrationFixture) settingsPath() string {
	return filepath.Join(f.agentDir, piSettingsFile)
}

func (f adapterMigrationFixture) mcpPath() string {
	return filepath.Join(f.agentDir, piMCPConfigFile)
}

func (f adapterMigrationFixture) opencodePath() string {
	return filepath.Join(f.opencodeDir, "opencode.json")
}

// writeJSONObject writes a pretty object and returns the exact bytes written.
func (f adapterMigrationFixture) writeJSONObject(t *testing.T, path string, object map[string]any) []byte {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	encoded, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		t.Fatalf("marshal %q: %v", path, err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
	return encoded
}

func readPiObject(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("unmarshal %q: %v", path, err)
	}
	return object
}

func piServers(t *testing.T, object map[string]any) map[string]any {
	t.Helper()
	servers, _ := object["mcpServers"].(map[string]any)
	if servers == nil {
		t.Fatalf("mcpServers missing: %v", object)
	}
	return servers
}

func piServerEntry(t *testing.T, object map[string]any, name string) map[string]any {
	t.Helper()
	servers := piServers(t, object)
	entry, _ := servers[name].(map[string]any)
	if entry == nil {
		t.Fatalf("mcpServers.%s missing: %v", name, servers)
	}
	return entry
}

func requireAbsent(t *testing.T, object map[string]any, key, context string) {
	t.Helper()
	if value, ok := object[key]; ok {
		t.Errorf("%s must not carry %q, got %v", context, key, value)
	}
}

func requireStringList(t *testing.T, value any, context string) []string {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("%s is not a JSON array: %v", context, value)
	}
	got := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("%s has a non-string element: %v", context, value)
		}
		got = append(got, text)
	}
	return got
}

func requireStringListEqual(t *testing.T, value any, context string, want ...string) {
	t.Helper()
	got := requireStringList(t, value, context)
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", context, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", context, got, want)
		}
	}
}

// TestMigrateMCP_OpenCodeServers covers the documented
// OpenCode → Pi conversion applied by the one-time migration: a local command
// array splits into command + args, environment becomes env, {env:NAME} becomes
// ${NAME}, a remote entry becomes a url entry, and OpenCode-only keys do not
// survive into the native entry.
func TestMigrateMCP_OpenCodeServers(t *testing.T) {
	f := newAdapterMigrationFixture(t)
	opencodeBefore := f.writeJSONObject(t, f.opencodePath(), map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"agent":   map[string]any{"build": map[string]any{"model": "opencode/model"}},
		"mcp": map[string]any{
			"context7": map[string]any{
				"type":        "local",
				"command":     []any{"npx", "-y", "@upstash/context7-mcp"},
				"environment": map[string]any{"CONTEXT7_API_KEY": "{env:CONTEXT7_API_KEY}"},
				"enabled":     true,
				"lifecycle":   "session",
				"directTools": []any{"resolve-library-id"},
			},
			"docs": map[string]any{
				"type":    "remote",
				"url":     "https://docs.example.com/mcp",
				"headers": map[string]any{"Authorization": "Bearer {env:DOCS_TOKEN}"},
			},
		},
	})

	changed, _, err := NewAdapter().ProvisionBigMemMCP(f.home)
	if err != nil {
		t.Fatalf("ProvisionBigMemMCP: %v", err)
	}
	if !changed {
		t.Error("first provision with un-migrated servers must report changed=true")
	}

	mcp := readPiObject(t, f.mcpPath())

	context7 := piServerEntry(t, mcp, "context7")
	if context7["command"] != "npx" {
		t.Errorf("context7.command = %v, want npx (command array must split)", context7["command"])
	}
	requireStringListEqual(t, context7["args"], "context7.args", "-y", "@upstash/context7-mcp")
	env, _ := context7["env"].(map[string]any)
	if env == nil || env["CONTEXT7_API_KEY"] != "${CONTEXT7_API_KEY}" {
		t.Errorf("context7.env = %v, want CONTEXT7_API_KEY=${CONTEXT7_API_KEY}", context7["env"])
	}
	if context7["enabled"] != true {
		t.Errorf("context7.enabled = %v, want true (native key preserved)", context7["enabled"])
	}
	for _, key := range []string{"type", "environment", "lifecycle", "directTools"} {
		requireAbsent(t, context7, key, "migrated context7")
	}

	docs := piServerEntry(t, mcp, "docs")
	if docs["url"] != "https://docs.example.com/mcp" {
		t.Errorf("docs.url = %v, want the remote url", docs["url"])
	}
	headers, _ := docs["headers"].(map[string]any)
	if headers == nil || headers["Authorization"] != "Bearer ${DOCS_TOKEN}" {
		t.Errorf("docs.headers = %v, want Authorization=Bearer ${DOCS_TOKEN}", docs["headers"])
	}
	requireAbsent(t, docs, "type", "migrated docs")

	// The source config is read-only: the migration never rewrites it.
	opencodeAfter, err := os.ReadFile(f.opencodePath())
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	if string(opencodeBefore) != string(opencodeAfter) {
		t.Error("opencode.json was rewritten by the migration")
	}
}

// TestMigrateMCP_SettingsMCPServers covers the second source:
// foreign entries left in the adapter-era settings.json `mcpServers` map are
// folded into mcp.json, every unrelated key in settings.json survives, and the
// biggz-owned `bigmem` entry is skipped rather than migrated.
func TestMigrateMCP_SettingsMCPServers(t *testing.T) {
	f := newAdapterMigrationFixture(t)
	f.writeJSONObject(t, f.settingsPath(), map[string]any{
		"mcpServers": map[string]any{
			"other": map[string]any{
				"type":        "local",
				"command":     []any{"other-cmd", "--flag"},
				"environment": map[string]any{"TOKEN": "{env:OTHER_TOKEN}"},
				"directTools": []any{"x"},
			},
			"bigmem": map[string]any{"command": "legacy-biggz-mcp", "type": "local"},
		},
		"directTools":  []any{"biggz_mem_save"},
		"defaultTools": []any{"read"},
		"packages":     []any{"npm:x"},
	})

	if _, _, err := NewAdapter().ProvisionBigMemMCP(f.home); err != nil {
		t.Fatalf("ProvisionBigMemMCP: %v", err)
	}

	mcp := readPiObject(t, f.mcpPath())
	other := piServerEntry(t, mcp, "other")
	if other["command"] != "other-cmd" {
		t.Errorf("other.command = %v, want other-cmd", other["command"])
	}
	requireStringListEqual(t, other["args"], "other.args", "--flag")
	env, _ := other["env"].(map[string]any)
	if env == nil || env["TOKEN"] != "${OTHER_TOKEN}" {
		t.Errorf("other.env = %v, want TOKEN=${OTHER_TOKEN}", other["env"])
	}
	for _, key := range []string{"type", "environment", "directTools"} {
		requireAbsent(t, other, key, "migrated other")
	}

	// bigmem is biggz-owned: mcp.json carries the native entry, never the
	// settings.json leftover.
	bigmem := piServerEntry(t, mcp, "bigmem")
	if bigmem["command"] == "legacy-biggz-mcp" {
		t.Error("settings.json bigmem must not be migrated into mcp.json")
	}
	requireStringListEqual(t, bigmem["args"], "bigmem.args", "--tools=agent", "--prefix=biggz")

	settings := readPiObject(t, f.settingsPath())
	servers, _ := settings["mcpServers"].(map[string]any)
	if _, ok := servers["bigmem"]; ok {
		t.Error("settings.json kept the biggz-owned mcpServers.bigmem")
	}
	if _, ok := servers["other"]; !ok {
		t.Error("settings.json lost the foreign mcpServers.other")
	}
	if _, ok := settings["defaultTools"]; !ok {
		t.Error("settings.json lost the foreign defaultTools key")
	}
	if _, ok := settings["packages"]; !ok {
		t.Error("settings.json lost the packages reconcile")
	}
}

// TestMigrateMCP_PreservesExistingNames covers the
// non-destructive half of the contract: a name mcp.json already defines stays
// exactly as it is, and unrelated keys in every file survive the pass.
func TestMigrateMCP_PreservesExistingNames(t *testing.T) {
	f := newAdapterMigrationFixture(t)
	f.writeJSONObject(t, f.mcpPath(), map[string]any{
		"mcpServers": map[string]any{
			"context7": map[string]any{"command": "native-keep", "args": []any{"a"}},
		},
		"theme":   "dark",
		"imports": []any{"opencode"},
	})
	f.writeJSONObject(t, f.opencodePath(), map[string]any{
		"mcp": map[string]any{
			"context7": map[string]any{"type": "local", "command": []any{"npx", "context7"}},
			"fresh":    map[string]any{"type": "local", "command": []any{"fresh-cmd"}},
		},
	})

	if _, _, err := NewAdapter().ProvisionBigMemMCP(f.home); err != nil {
		t.Fatalf("ProvisionBigMemMCP: %v", err)
	}

	mcp := readPiObject(t, f.mcpPath())
	context7 := piServerEntry(t, mcp, "context7")
	if context7["command"] != "native-keep" {
		t.Errorf("existing context7.command = %v, want native-keep (never overwritten)", context7["command"])
	}
	requireStringListEqual(t, context7["args"], "existing context7.args", "a")
	if mcp["theme"] != "dark" {
		t.Errorf("foreign top-level mcp.json key lost: %v", mcp["theme"])
	}
	fresh := piServerEntry(t, mcp, "fresh")
	if fresh["command"] != "fresh-cmd" {
		t.Errorf("fresh.command = %v, want fresh-cmd (absent name must be added)", fresh["command"])
	}
}

// TestMigrateMCP_IdempotentAndByteStable proves a second run is a
// fixed point: nothing is re-added, nothing is rewritten, reported changed=false.
func TestMigrateMCP_IdempotentAndByteStable(t *testing.T) {
	f := newAdapterMigrationFixture(t)
	f.writeJSONObject(t, f.mcpPath(), map[string]any{
		"mcpServers": map[string]any{"native": map[string]any{"command": "native"}},
		"theme":      "dark",
	})
	f.writeJSONObject(t, f.opencodePath(), map[string]any{
		"mcp": map[string]any{
			"context7": map[string]any{"type": "local", "command": []any{"npx", "context7"}},
		},
	})
	f.writeJSONObject(t, f.settingsPath(), map[string]any{
		"mcpServers": map[string]any{
			"other": map[string]any{"type": "local", "command": []any{"other-cmd"}},
		},
	})

	a := NewAdapter()
	if _, _, err := a.ProvisionBigMemMCP(f.home); err != nil {
		t.Fatalf("first provision: %v", err)
	}
	settingsBefore, _ := os.ReadFile(f.settingsPath())
	mcpBefore, _ := os.ReadFile(f.mcpPath())

	changed, _, err := a.ProvisionBigMemMCP(f.home)
	if err != nil {
		t.Fatalf("second provision: %v", err)
	}
	if changed {
		t.Error("second provision must report changed=false")
	}
	settingsAfter, _ := os.ReadFile(f.settingsPath())
	mcpAfter, _ := os.ReadFile(f.mcpPath())
	if string(settingsBefore) != string(settingsAfter) {
		t.Error("settings.json changed on the idempotent second run")
	}
	if string(mcpBefore) != string(mcpAfter) {
		t.Error("mcp.json changed on the idempotent second run")
	}
}

// TestMigrateMCP_ConvertEntry pins the OpenCode → Pi entry conversion (the
// documented table in pi's docs/mcp.md) plus the dropped-key reporting that
// makes the pass observable.
func TestMigrateMCP_ConvertEntry(t *testing.T) {
	tests := []struct {
		name    string
		raw     map[string]any
		want    map[string]any
		dropped []string
		ok      bool
	}{
		{
			name:    "local command array splits into command and args",
			raw:     map[string]any{"type": "local", "command": []any{"npx", "-y", "srv"}},
			want:    map[string]any{"command": "npx", "args": []string{"-y", "srv"}},
			dropped: []string{"type"},
			ok:      true,
		},
		{
			name:    "environment becomes env",
			raw:     map[string]any{"command": "srv", "environment": map[string]any{"A": "b"}},
			want:    map[string]any{"command": "srv", "env": map[string]any{"A": "b"}},
			dropped: []string{"environment"},
			ok:      true,
		},
		{
			name: "env placeholders become dollar-brace",
			raw: map[string]any{
				"command":     []any{"tool", "--token", "{env:TOK}"},
				"environment": map[string]any{"K": "{env:X}"},
			},
			want:    map[string]any{"command": "tool", "args": []string{"--token", "${TOK}"}, "env": map[string]any{"K": "${X}"}},
			dropped: []string{"environment"},
			ok:      true,
		},
		{
			name: "remote becomes a url entry",
			raw: map[string]any{
				"type":    "remote",
				"url":     "https://docs.example.com/mcp",
				"headers": map[string]any{"Authorization": "Bearer {env:T}"},
			},
			want:    map[string]any{"url": "https://docs.example.com/mcp", "headers": map[string]any{"Authorization": "Bearer ${T}"}},
			dropped: []string{"type"},
			ok:      true,
		},
		{
			name:    "adapter-only keys are dropped and reported",
			raw:     map[string]any{"command": "srv", "type": "local", "directTools": []any{"a"}, "lifecycle": "session", "userApiKey": "sk-live"},
			want:    map[string]any{"command": "srv"},
			dropped: []string{"directTools", "lifecycle", "type", "userApiKey"},
			ok:      true,
		},
		{
			name:    "entry without command or url is refused",
			raw:     map[string]any{"type": "local", "environment": map[string]any{"A": "b"}},
			want:    nil,
			dropped: []string{"environment", "type"},
			ok:      false,
		},
		{
			name:    "illegal env values are dropped and reported",
			raw:     map[string]any{"command": "srv", "environment": map[string]any{"OK": "v", "BAD": 42}},
			want:    map[string]any{"command": "srv", "env": map[string]any{"OK": "v"}},
			dropped: []string{"env.BAD", "environment"},
			ok:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, dropped, ok := convertMCPServerEntry(tt.raw)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v (entry %v)", ok, tt.ok, got)
			}
			if !ok {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("entry = %#v, want %#v", got, tt.want)
			}
			if !reflect.DeepEqual(dropped, tt.dropped) {
				t.Errorf("dropped = %v, want %v", dropped, tt.dropped)
			}
		})
	}
}

// TestMigrateMCP_ReportsMigratedNames pins the observability
// contract: the provisioning report names every server the pass folded into
// mcp.json and never names a skipped or already-present one.
func TestMigrateMCP_ReportsMigratedNames(t *testing.T) {
	f := newAdapterMigrationFixture(t)
	f.writeJSONObject(t, f.settingsPath(), map[string]any{
		"mcpServers": map[string]any{
			"other":  map[string]any{"type": "local", "command": []any{"other-cmd"}},
			"bigmem": map[string]any{"command": "legacy-biggz-mcp", "type": "local"},
		},
	})
	f.writeJSONObject(t, f.opencodePath(), map[string]any{
		"mcp": map[string]any{
			"context7": map[string]any{"type": "local", "command": []any{"npx", "context7"}},
			"docs":     map[string]any{"type": "remote", "url": "https://docs.example.com/mcp"},
			"other":    map[string]any{"type": "local", "command": []any{"shadowed"}},
		},
	})
	f.writeJSONObject(t, f.mcpPath(), map[string]any{
		"mcpServers": map[string]any{"native": map[string]any{"command": "native"}},
	})

	report, err := NewAdapter().ProvisionBigMemMCPReport(f.home)
	if err != nil {
		t.Fatalf("ProvisionBigMemMCPReport: %v", err)
	}
	if !report.Changed {
		t.Error("report.Changed = false, want true (servers were folded in)")
	}
	if !report.Migration.Changed {
		t.Error("report.Migration.Changed = false, want true")
	}
	names := make([]string, 0, len(report.Migration.Migrated))
	for _, migration := range report.Migration.Migrated {
		names = append(names, migration.Name)
	}
	slices.Sort(names)
	if want := []string{"context7", "docs", "other"}; !slices.Equal(names, want) {
		t.Errorf("migrated names = %v, want %v", names, want)
	}
	if len(report.Files) != 2 {
		t.Errorf("report.Files = %v, want the two provisioned paths", report.Files)
	}
	for _, migration := range report.Migration.Migrated {
		if migration.Source == "" {
			t.Errorf("migration %q reports no source", migration.Name)
		}
	}

	// Nothing left to fold in: the second report must be empty and unchanged.
	// `other` arrives from settings.json first, so opencode's shadowing copy is
	// never reported as migrated twice.
	second, err := NewAdapter().ProvisionBigMemMCPReport(f.home)
	if err != nil {
		t.Fatalf("second ProvisionBigMemMCPReport: %v", err)
	}
	if second.Changed || second.Migration.Changed || len(second.Migration.Migrated) != 0 {
		t.Errorf("second report = %+v, want an unchanged no-op", second)
	}
}

// TestMigrateMCP_UnreadableSourceDoesNotBlock covers the best-effort
// half of the pass: a third-party config biggz does not own can never block Pi's
// native provisioning. The file is reported as unreadable and left untouched.
func TestMigrateMCP_UnreadableSourceDoesNotBlock(t *testing.T) {
	f := newAdapterMigrationFixture(t)
	broken := []byte("{\n  // comment\n  \"mcp\": { broken\n}\n")
	if err := os.MkdirAll(f.opencodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.opencodePath(), broken, 0o644); err != nil {
		t.Fatal(err)
	}
	f.writeJSONObject(t, f.settingsPath(), map[string]any{
		"mcpServers": map[string]any{
			"other": map[string]any{"type": "local", "command": []any{"other-cmd"}},
		},
	})

	report, err := NewAdapter().ProvisionBigMemMCPReport(f.home)
	if err != nil {
		t.Fatalf("ProvisionBigMemMCPReport must tolerate an unparseable import source: %v", err)
	}
	if len(report.Migration.Unreadable) != 1 {
		t.Fatalf("unreadable sources = %v, want exactly the opencode config", report.Migration.Unreadable)
	}
	if !strings.Contains(report.Migration.Unreadable[0], f.opencodePath()) {
		t.Errorf("unreadable entry %q does not name %q", report.Migration.Unreadable[0], f.opencodePath())
	}
	if got := report.Migration.MigratedNames(); !slices.Equal(got, []string{"other"}) {
		t.Errorf("migrated = %v, want [other] from settings.json", got)
	}
	after, err := os.ReadFile(f.opencodePath())
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	if string(broken) != string(after) {
		t.Error("unreadable source was modified")
	}
}

// TestMigrateMCP_OpenCodeMCPServersKey covers the secondary key
// the adapter-era OpenCode config could use for the same map.
func TestMigrateMCP_OpenCodeMCPServersKey(t *testing.T) {
	f := newAdapterMigrationFixture(t)
	f.writeJSONObject(t, f.opencodePath(), map[string]any{
		"mcpServers": map[string]any{
			"legacy": map[string]any{"type": "local", "command": []any{"legacy-cmd", "--k"}},
		},
	})

	report, err := NewAdapter().ProvisionBigMemMCPReport(f.home)
	if err != nil {
		t.Fatalf("ProvisionBigMemMCPReport: %v", err)
	}
	if got := report.Migration.MigratedNames(); !slices.Equal(got, []string{"legacy"}) {
		t.Fatalf("migrated = %v, want [legacy]", got)
	}
	entry := piServerEntry(t, readPiObject(t, f.mcpPath()), "legacy")
	if entry["command"] != "legacy-cmd" {
		t.Errorf("legacy.command = %v, want legacy-cmd", entry["command"])
	}
	requireStringListEqual(t, entry["args"], "legacy.args", "--k")
}
