package orchestrator

import (
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const WRITER_EDIT_SURFACE_REJECTION = "Parent must derive or map narrow repository-relative allowed edit surfaces from the delegated task and relaunch the writer. Do not ask the human to author paths or globs."

var (
	boundedWriterAgents          = map[string]bool{"gentle-ai-worker": true, "worker": true}
	allowedEditSurfacesHeadingRe = regexp.MustCompile(`(?mi)^## Allowed edit surfaces[ \t]*$`)
	headingRe                    = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	markdownListMarkerRe         = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s+`)
	whitespaceRe                 = regexp.MustCompile(`\s`)
)

func isTaskScopedRepositoryRelativePath(value string) bool {
	n := strings.ReplaceAll(value, "\\", "/")
	if len(n) == 0 || filepath.IsAbs(value) || filepath.IsAbs(n) {
		return false
	}
	if ok, _ := regexp.MatchString(`^(?:[A-Za-z]:|/|~)`, n); ok {
		return false
	}
	w := regexp.MustCompile(`^(?:\./)+`).ReplaceAllString(n, "")
	if len(w) == 0 || w == "." || strings.HasPrefix(w, "/") {
		return false
	}
	if whitespaceRe.MatchString(w) {
		return false
	}
	for _, s := range strings.Split(w, "/") {
		if s == ".." {
			return false
		}
	}
	first := strings.Split(w, "/")[0]
	return !strings.ContainsAny(first, "?*[]{}")
}

func IsTaskScopedRepositoryRelativePath(v string) bool { return isTaskScopedRepositoryRelativePath(v) }

func readSurfaceEntry(line string) string {
	entry := markdownListMarkerRe.ReplaceAllString(line, "")
	if len(entry) >= 2 && strings.HasPrefix(entry, "`") && strings.HasSuffix(entry, "`") {
		return entry[1 : len(entry)-1]
	}
	return entry
}

func looksLikeSurfaceEntry(line string) bool {
	if len(line) == 0 {
		return false
	}
	if headingRe.MatchString(line) {
		return false
	}
	return !whitespaceRe.MatchString(readSurfaceEntry(line))
}

func readAllowedEditSurfaceEntries(following string) []string {
	linesRaw := strings.Split(following, "\n")
	lines := make([]string, len(linesRaw))
	for i, l := range linesRaw {
		// trim \r for \r\n
		l = strings.TrimSuffix(l, "\r")
		lines[i] = strings.TrimSpace(l)
	}
	headingIdx := -1
	for i, l := range lines {
		if headingRe.MatchString(l) {
			headingIdx = i
			break
		}
	}
	section := lines
	if headingIdx != -1 {
		section = lines[:headingIdx]
	}
	entries := []string{}
	for idx, line := range section {
		if len(line) == 0 {
			continue
		}
		entry := readSurfaceEntry(line)
		if whitespaceRe.MatchString(entry) {
			// prose closes list only when genuinely trailing; otherwise validate all
			hasLaterEntry := false
			for _, cand := range section[idx+1:] {
				if looksLikeSurfaceEntry(cand) {
					hasLaterEntry = true
					break
				}
			}
			if hasLaterEntry {
				// ambiguous: return all non-empty read entries for validation (will fail)
				var all []string
				for _, cand := range section {
					if len(cand) == 0 {
						continue
					}
					all = append(all, readSurfaceEntry(cand))
				}
				return all
			}
			break
		}
		entries = append(entries, entry)
	}
	return entries
}

// ReadAllowedEditSurfaceEntries is exported for parity harness and tests.
func ReadAllowedEditSurfaceEntries(section string) []string {
	return readAllowedEditSurfaceEntries(section)
}

func hasTaskScopedAllowedEditSurfaces(values ...string) bool {
	var exp []string
	has := false
	for _, v := range values {
		matches := allowedEditSurfacesHeadingRe.FindAllStringIndex(v, -1)
		for _, loc := range matches {
			following := v[loc[1]:]
			entries := readAllowedEditSurfaceEntries(following)
			if len(entries) == 0 {
				return false
			}
			for _, e := range entries {
				if !isTaskScopedRepositoryRelativePath(e) {
					return false
				}
			}
			uniq := dedupSort(entries)
			if exp != nil && (len(exp) != len(uniq) || !equalSorted(exp, uniq)) {
				return false
			}
			exp = uniq
			has = true
		}
	}
	return has
}

func HasTaskScopedAllowedEditSurfaces(v ...string) bool {
	return hasTaskScopedAllowedEditSurfaces(v...)
}

func dedupSort(in []string) []string {
	m := map[string]struct{}{}
	for _, v := range in {
		m[v] = struct{}{}
	}
	o := make([]string, 0, len(m))
	for k := range m {
		o = append(o, k)
	}
	sort.Strings(o)
	return o
}

func equalSorted(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// GuardSDAgentDispatch wires SD Agent Authority into orchestrator surfaces.
// SDD phases must use sdd-* agents only; general/explore for SDD is rejected fail-closed.
func GuardSDAgentDispatch(phase, agent string) *Rejection {
	if err := GuardSDAgentAuthority(phase, agent); err != nil {
		return &Rejection{Block: true, Reason: err.Error()}
	}
	return nil
}

type Rejection struct {
	Block  bool
	Reason string
}

func rejectUnscopedBoundedWriterDispatch(input map[string]any) *Rejection {
	if input == nil {
		return nil
	}
	agent, _ := input["agent"].(string)
	if !boundedWriterAgents[agent] {
		return nil
	}
	task, _ := input["task"].(string)
	ctx, _ := input["context"].(string)
	if hasTaskScopedAllowedEditSurfaces(task, ctx) {
		return nil
	}
	log.Printf("[orchestrator] scout_fallback agent=%s reason=%s Block=true", agent, WRITER_EDIT_SURFACE_REJECTION)
	return &Rejection{Block: true, Reason: WRITER_EDIT_SURFACE_REJECTION}
}

func RejectUnscopedBoundedWriterDispatch(input map[string]any) *Rejection {
	return rejectUnscopedBoundedWriterDispatch(input)
}
