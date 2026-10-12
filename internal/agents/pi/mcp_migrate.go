package pi

import (
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"

	opencodeadapter "github.com/biggs-100/biggz-ai/internal/agents/opencode"
	"github.com/biggs-100/biggz-ai/internal/filemerge"
)

// This file is the one-time adapter migration (unit 1.2). The retired
// pi-mcp-adapter reached MCP servers through `imports: ["opencode"]` and through
// the `mcpServers` map an earlier install left in Pi's settings.json. Native Pi
// reads only ~/.pi/agent/mcp.json, so both sources are folded into it before the
// adapter-era keys are pruned, without ever overwriting a server that mcp.json
// already defines.
//
// The conversion is the documented OpenCode -> Pi one (pi docs/mcp.md
// "Migrate configuration from another client"): a "local" entry's command array
// splits into command + args, `environment` becomes `env`, `{env:NAME}` becomes
// `${NAME}`, and a "remote" entry becomes a `url` entry.

// piNativeMCPServerKeys are the entry keys Pi's mcp.json accepts (pi
// docs/mcp.md "Configure servers"). Everything else an adapter-era config
// carries is OpenCode/adapter vocabulary and is dropped, so a converted entry
// can never make Pi report a config error. `type` is deliberately absent: Pi
// infers stdio from `command` and streamable HTTP from `url` (decision D6).
var piNativeMCPServerKeys = map[string]struct{}{
	"command":      {},
	"args":         {},
	"env":          {},
	"cwd":          {},
	"url":          {},
	"headers":      {},
	"oauth":        {},
	"timeout":      {},
	"enabled":      {},
	"exposure":     {},
	"toolExposure": {},
	"description":  {},
}

// piMCPServerNamePattern is Pi's server-name grammar: letters, digits, "_" and
// "-" (pi docs/mcp.md "Configuration rules"). An imported name outside it would
// be rejected by Pi, so the migration refuses it and reports it instead.
var piMCPServerNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// openCodeEnvPlaceholderPattern matches OpenCode's `{env:NAME}` placeholder.
var openCodeEnvPlaceholderPattern = regexp.MustCompile(`\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)

// mcpImportSource is one config file whose declared MCP servers are imported
// into Pi's native mcp.json. keys lists the candidate top-level maps in
// priority order (OpenCode declares servers under "mcp"); skip names servers the
// caller owns and must not import.
type mcpImportSource struct {
	path string
	keys []string
	skip map[string]struct{}
}

// MCPServerMigration records one server the migration pass saw in a source.
type MCPServerMigration struct {
	Name   string
	Source string
	// Dropped lists the source keys that were not carried into the native
	// entry: OpenCode/adapter vocabulary, illegal env values, or — for a
	// skipped entry — the reason it was refused.
	Dropped []string
}

// MCPMigrationReport is the observable outcome of one reconciling migration
// pass. Migrated names every server folded into mcp.json, Skipped names every
// entry Pi cannot accept, and Unreadable lists each source file that could not
// be parsed as "<path>: <reason>" — such a file is left alone and never blocks
// the pass.
type MCPMigrationReport struct {
	Changed    bool
	Migrated   []MCPServerMigration
	Skipped    []MCPServerMigration
	Unreadable []string
}

// MigratedNames returns the names folded into mcp.json, sorted.
func (r MCPMigrationReport) MigratedNames() []string {
	names := make([]string, 0, len(r.Migrated))
	for _, migration := range r.Migrated {
		names = append(names, migration.Name)
	}
	slices.Sort(names)
	return names
}

// importedMCPSources are the adapter-era configs that fed the retired adapter:
// Pi's settings.json `mcpServers` map (minus the biggz-owned bigmem entry, which
// unit 1.1 prunes and this unit must not resurrect) and the OpenCode global
// config. The OpenCode path comes from the repository's own resolver
// (internal/agents/opencode SettingsPath: ~/.config/opencode/opencode.json), so
// pi does not invent a second location for it.
func importedMCPSources(homeDir, settingsPath string) []mcpImportSource {
	return []mcpImportSource{
		{
			path: settingsPath,
			keys: []string{"mcpServers"},
			skip: map[string]struct{}{"bigmem": {}},
		},
		{
			path: opencodeadapter.NewAdapter().SettingsPath(homeDir),
			keys: []string{"mcp", "mcpServers"},
		},
	}
}

// migrateMCPServers folds every server declared by sources into servers, adding
// a server only when its name is still absent, so an existing mcp.json entry is
// never overwritten or removed. It is a fixed point: a second pass over the same
// sources adds nothing, which keeps the caller's write byte-stable. An
// unparseable source is reported as unreadable instead of failing the pass.
func migrateMCPServers(servers map[string]any, sources []mcpImportSource) MCPMigrationReport {
	report := MCPMigrationReport{}
	for _, source := range sources {
		declared, err := readDeclaredMCPServers(source)
		if err != nil {
			report.Unreadable = append(report.Unreadable, fmt.Sprintf("%s: %v", source.path, err))
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(declared)) {
			if !piMCPServerNamePattern.MatchString(name) {
				report.Skipped = append(report.Skipped, MCPServerMigration{
					Name:    name,
					Source:  source.path,
					Dropped: []string{"invalid server name"},
				})
				continue
			}
			if _, exists := servers[name]; exists {
				continue
			}
			raw, _ := declared[name].(map[string]any)
			entry, dropped, ok := convertMCPServerEntry(raw)
			if !ok {
				report.Skipped = append(report.Skipped, MCPServerMigration{
					Name:    name,
					Source:  source.path,
					Dropped: dropped,
				})
				continue
			}
			servers[name] = entry
			report.Migrated = append(report.Migrated, MCPServerMigration{
				Name:    name,
				Source:  source.path,
				Dropped: dropped,
			})
		}
	}
	report.Changed = len(report.Migrated) > 0
	return report
}

// readDeclaredMCPServers reads one source's declared servers. A missing or empty
// file declares nothing. The JSONC-safe reader is used because the file is only
// ever read, never written back.
func readDeclaredMCPServers(source mcpImportSource) (map[string]any, error) {
	declared := map[string]any{}
	data, err := os.ReadFile(source.path)
	if err != nil {
		if os.IsNotExist(err) {
			return declared, nil
		}
		return nil, fmt.Errorf("read: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return declared, nil
	}
	root, err := filemerge.UnmarshalJSONObject(data)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	for _, key := range source.keys {
		servers, ok := root[key].(map[string]any)
		if !ok {
			continue
		}
		for name, raw := range servers {
			if _, skip := source.skip[name]; skip {
				continue
			}
			if _, seen := declared[name]; seen {
				// Earlier keys in the source win; a name is never imported twice.
				continue
			}
			declared[name] = raw
		}
	}
	return declared, nil
}

// convertMCPServerEntry maps one adapter-era server entry to Pi's native
// mcp.json shape (the documented OpenCode -> Pi conversion). ok is false when the
// entry cannot become a native server: it is not an object, it carries neither a
// command nor a url, or its command array is empty or non-string. dropped names
// every source key that was not carried over, sorted for stable reporting.
func convertMCPServerEntry(raw map[string]any) (map[string]any, []string, bool) {
	if raw == nil {
		return nil, nil, false
	}
	entry := make(map[string]any, len(raw))
	dropped := make([]string, 0, len(raw))

	for key, value := range raw {
		switch {
		case key == "type":
			// Pi infers the transport; "local" is a hard config error (E2).
			dropped = append(dropped, key)
		case key == "environment":
			env, refused, ok := stringEnv(value)
			if !ok {
				dropped = append(dropped, key)
				continue
			}
			entry["env"] = env
			dropped = append(dropped, refused...)
			dropped = append(dropped, key)
		case isNativeMCPServerKey(key):
			entry[key] = value
		default:
			dropped = append(dropped, key)
		}
	}

	if url, ok := entry["url"].(string); ok && url != "" {
		// A remote entry carries a url and never a command (pi docs/mcp.md): a
		// command left beside it would select the wrong transport.
		for _, key := range []string{"command", "args"} {
			if _, present := entry[key]; present {
				delete(entry, key)
				dropped = append(dropped, key)
			}
		}
	} else {
		delete(entry, "url")
		command, args, droppedCommand, ok := nativeCommandAndArgs(entry)
		if !ok {
			slices.Sort(dropped)
			return nil, dropped, false
		}
		entry["command"] = command
		if args != nil {
			entry["args"] = args
		} else {
			delete(entry, "args")
		}
		dropped = append(dropped, droppedCommand...)
	}

	// OpenCode writes {env:NAME}; Pi expands ${NAME}. Applied to every string in
	// the entry, so headers, oauth fields and command arguments all convert.
	entry = substituteOpenCodeEnvPlaceholders(entry).(map[string]any)
	slices.Sort(dropped)
	return entry, dropped, true
}

// nativeCommandAndArgs splits the adapter-era command into Pi's `command` (one
// executable) plus `args`. A command array is the OpenCode shape; a plain string
// is already native.
func nativeCommandAndArgs(entry map[string]any) (command string, args []string, dropped []string, ok bool) {
	switch value := entry["command"].(type) {
	case string:
		if value == "" {
			return "", nil, nil, false
		}
		command = value
	case []any:
		parts, stringy := stringList(value)
		if !stringy || len(parts) == 0 || parts[0] == "" {
			return "", nil, nil, false
		}
		command = parts[0]
		args = append(args, parts[1:]...)
	default:
		return "", nil, nil, false
	}

	if existing, present := entry["args"]; present {
		extra, stringy := valueAsStringList(existing)
		if !stringy {
			dropped = append(dropped, "args")
			return command, args, dropped, true
		}
		args = append(args, extra...)
	}
	return command, args, dropped, true
}

// stringEnv converts an adapter-era environment map into Pi's `env`. Only string
// values are legal env values, so an object or number holding a secret is
// refused and reported as "env.<KEY>" instead of being written.
func stringEnv(value any) (env map[string]any, refused []string, ok bool) {
	source, ok := value.(map[string]any)
	if !ok {
		return nil, nil, false
	}
	env = make(map[string]any, len(source))
	for key, raw := range source {
		text, isString := raw.(string)
		if !isString {
			refused = append(refused, "env."+key)
			continue
		}
		env[key] = text
	}
	slices.Sort(refused)
	return env, refused, true
}

// substituteOpenCodeEnvPlaceholders rewrites `{env:NAME}` to `${NAME}` in every
// string of a converted entry, including nested headers and oauth values.
func substituteOpenCodeEnvPlaceholders(value any) any {
	switch typed := value.(type) {
	case string:
		return openCodeEnvPlaceholderPattern.ReplaceAllString(typed, "${$1}")
	case []string:
		out := make([]string, len(typed))
		for i, item := range typed {
			out[i] = openCodeEnvPlaceholderPattern.ReplaceAllString(item, "${$1}")
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = substituteOpenCodeEnvPlaceholders(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = substituteOpenCodeEnvPlaceholders(item)
		}
		return out
	default:
		return value
	}
}

func isNativeMCPServerKey(key string) bool {
	_, ok := piNativeMCPServerKeys[key]
	return ok
}

// stringList accepts a JSON array of strings and rejects anything else.
func stringList(value []any) ([]string, bool) {
	items := make([]string, 0, len(value))
	for _, item := range value {
		text, ok := item.(string)
		if !ok {
			return nil, false
		}
		items = append(items, text)
	}
	return items, true
}

// valueAsStringList accepts a JSON array of strings in either its []any or
// []string form.
func valueAsStringList(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return typed, true
	case []any:
		return stringList(typed)
	default:
		return nil, false
	}
}
