package steps

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/filemerge"
)

func generateOverlay(fsys fs.FS, homeDir string) ([]byte, error) {
	if fsys == nil {
		fsys = assets.FS
	}
	data, err := fs.ReadFile(fsys, "opencode/sdd-overlay-multi.json")
	if err != nil {
		return nil, err
	}
	var overlay map[string]any
	if err := json.Unmarshal(data, &overlay); err != nil {
		return nil, fmt.Errorf("parse overlay: %w", err)
	}
	if agents, ok := overlay["agent"].(map[string]any); ok {
		if orch, ok := agents["biggz-orchestrator"].(map[string]any); ok {
			if prompt, ok := orch["prompt"].(string); ok && prompt == "__ORCHESTRATOR_PROMPT__" {
				if pdata, err := fs.ReadFile(fsys, "biggz/biggz-orchestrator.md"); err == nil {
					orch["prompt"] = string(pdata)
				}
			}
		}
	}
	_ = homeDir
	return json.MarshalIndent(overlay, "", "  ")
}

func extractMarkerContent(raw, name string) string {
	open := "<!-- " + name + " -->"
	close := "<!-- /" + name + " -->"
	s := strings.Index(raw, open)
	if s == -1 {
		return strings.TrimSpace(raw)
	}
	e := strings.Index(raw[s+len(open):], close)
	if e == -1 {
		return strings.TrimSpace(raw)
	}
	return strings.TrimSpace(raw[s+len(open) : s+len(open)+e])
}

func injectByMarker(existing, content, name string) string {
	open := "<!-- " + name + " -->"
	close := "<!-- /" + name + " -->"
	idx := strings.Index(existing, open)
	if idx == -1 {
		if existing != "" && !strings.HasSuffix(existing, "\n") {
			existing += "\n"
		}
		return existing + "\n" + open + "\n" + content + "\n" + close + "\n"
	}
	cidx := strings.Index(existing[idx+len(open):], close)
	if cidx == -1 {
		return existing[:idx] + open + "\n" + content + "\n" + close + "\n"
	}
	cidx += idx + len(open)
	return existing[:idx] + open + "\n" + content + "\n" + close + existing[cidx+len(close):]
}
func piAgentsDir(homeDir string) string {
	if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); v != "" {
		return filepath.Join(v, "agents")
	}
	return filepath.Join(homeDir, ".pi", "agent", "agents")
}
func piExtensionsDir(homeDir string) string {
	if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); v != "" {
		return filepath.Join(v, "extensions")
	}
	return filepath.Join(homeDir, ".pi", "agent", "extensions")
}
func mergeJSONCWrapper(existing, overlay []byte) ([]byte, error) {
	return filemerge.MergeJSONC(existing, overlay)
}
func parseFrontmatter(data string) (string, string, string, error) {
	section := data
	hasCapable := false
	capableStart := -1
	if s := strings.Index(data, "<!-- section:model-capable -->"); s != -1 {
		capableStart = s
		if e := strings.Index(data, "<!-- /section:model-capable -->"); e != -1 && e > s {
			section = data[s+len("<!-- section:model-capable -->") : e]
			hasCapable = true
		}
	}
	lines := strings.Split(section, "\n")
	startIdx, endIdx := -1, -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if startIdx == -1 {
				startIdx = i
			} else {
				endIdx = i
				break
			}
		}
	}
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		if hasCapable {
			prefix := data[:capableStart]
			plines := strings.Split(strings.TrimSpace(prefix), "\n")
			pStart, pEnd := -1, -1
			for i, line := range plines {
				if strings.TrimSpace(line) == "---" {
					if pStart == -1 {
						pStart = i
					} else {
						pEnd = i
						break
					}
				}
			}
			if pStart != -1 && pEnd != -1 && pEnd > pStart {
				pFront := plines[pStart+1 : pEnd]
				body := strings.TrimSpace(section)
				if idx := strings.Index(body, "<!-- section:model-small -->"); idx != -1 {
					body = strings.TrimSpace(body[:idx])
				}
				var name, desc string
				for i, line := range pFront {
					trim := strings.TrimSpace(line)
					if strings.HasPrefix(trim, "name:") {
						v := strings.TrimSpace(strings.TrimPrefix(trim, "name:"))
						name = strings.Trim(v, "\"'")
					} else if strings.HasPrefix(trim, "description:") {
						v := strings.TrimSpace(strings.TrimPrefix(trim, "description:"))
						v = strings.Trim(v, "\"'")
						if v == ">" || v == "|" {
							var parts []string
							for j := i + 1; j < len(pFront); j++ {
								nxt := pFront[j]
								if strings.HasPrefix(nxt, "  ") || strings.HasPrefix(nxt, "\t") {
									parts = append(parts, strings.TrimSpace(nxt))
								} else {
									break
								}
							}
							if len(parts) > 0 {
								desc = strings.Join(parts, " ")
							}
						} else {
							desc = v
						}
					}
				}
				return name, desc, body, nil
			}
		}
		return "", "", strings.TrimSpace(section), nil
	}
	frontLines := lines[startIdx+1 : endIdx]
	body := strings.TrimSpace(strings.Join(lines[endIdx+1:], "\n"))
	if idx := strings.Index(body, "<!-- section:model-small -->"); idx != -1 {
		body = strings.TrimSpace(body[:idx])
	}
	var name, desc string
	for i, line := range frontLines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "name:") {
			v := strings.TrimSpace(strings.TrimPrefix(trim, "name:"))
			name = strings.Trim(v, "\"'")
		} else if strings.HasPrefix(trim, "description:") {
			v := strings.TrimSpace(strings.TrimPrefix(trim, "description:"))
			v = strings.Trim(v, "\"'")
			if v == ">" || v == "|" {
				var parts []string
				for j := i + 1; j < len(frontLines); j++ {
					nxt := frontLines[j]
					if strings.HasPrefix(nxt, "  ") || strings.HasPrefix(nxt, "\t") {
						parts = append(parts, strings.TrimSpace(nxt))
					} else {
						break
					}
				}
				if len(parts) > 0 {
					desc = strings.Join(parts, " ")
				}
			} else {
				desc = v
			}
		}
	}
	return name, desc, body, nil
}
