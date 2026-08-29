package orchestrator

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const WRITER_EDIT_SURFACE_REJECTION = "Parent must derive or map narrow repository-relative allowed edit surfaces from the delegated task and relaunch the writer. Do not ask the human to author paths or globs."

var (
	boundedWriterAgents          = map[string]bool{"gentle-ai-worker": true, "worker": true}
	allowedEditSurfacesHeadingRe = regexp.MustCompile(`(?mi)^## Allowed edit surfaces[ \t]*$`)
	headingRe                    = regexp.MustCompile(`(?m)^#{1,2}\s+`)
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
	if strings.Contains(w, " ") || strings.Contains(w, "\t") || strings.Contains(w, "\n") {
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

func hasTaskScopedAllowedEditSurfaces(values ...string) bool {
	var exp []string
	has := false
	for _, v := range values {
		matches := allowedEditSurfacesHeadingRe.FindAllStringIndex(v, -1)
		for _, loc := range matches {
			following := v[loc[1]:]
			if idx := headingRe.FindStringIndex(following); idx != nil {
				following = following[:idx[0]]
			}
			entries := parseAllowedEntries(following)
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

func parseAllowedEntries(section string) []string {
	var out []string
	re := regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s+`)
	for _, line := range strings.Split(section, "\n") {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		s = re.ReplaceAllString(s, "")
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`") && len(s) >= 2 {
			s = s[1 : len(s)-1]
		}
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
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
	return &Rejection{Block: true, Reason: WRITER_EDIT_SURFACE_REJECTION}
}

func RejectUnscopedBoundedWriterDispatch(input map[string]any) *Rejection {
	return rejectUnscopedBoundedWriterDispatch(input)
}
