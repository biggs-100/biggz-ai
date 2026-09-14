package gitexec

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// LoadBaseline reads the frozen baseline (rule<TAB>path<TAB>line). A missing or
// unreadable baseline is a failure: absence is never read as "nothing to check".
func LoadBaseline(path string) ([]Finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []Finding
	line := 0
	for text := range strings.SplitSeq(string(data), "\n") {
		line++
		text = strings.TrimSpace(text)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) != 3 {
			return nil, fmt.Errorf("%s:%d: want rule<TAB>path<TAB>line, got %q", path, line, text)
		}
		n, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		entries = append(entries, Finding{Rule: Rule(fields[0]), Path: filepath.ToSlash(fields[1]), Line: n})
	}
	return entries, nil
}

// FormatBaseline renders entries as baseline lines in canonical order.
func FormatBaseline(entries []Finding) []byte {
	var out []byte
	for _, entry := range slices.SortedFunc(slices.Values(entries), compareFindings) {
		out = fmt.Appendf(out, "%s\n", entry)
	}
	return out
}

// siteKey groups sites by rule and path: entries are matched by count, never by line.
type siteKey struct {
	rule Rule
	path string
}

// Reconcile compares live findings against baseline entries grouped by (rule, path)
// count. Entries matching no live site are stale and live sites with no entry are
// added; both are failures. It also returns the entries with the line numbers of
// matched sites refreshed. Reconcile never adds or deletes entries: a human deletes
// an entry together with the migration that removed its site.
func Reconcile(entries, findings []Finding) (stale, added, refreshed []Finding) {
	live := map[siteKey][]Finding{}
	for _, f := range findings {
		key := siteKey{f.Rule, f.Path}
		live[key] = append(live[key], f)
	}
	used := map[siteKey]int{}
	refreshed = make([]Finding, 0, len(entries))
	for _, entry := range entries {
		key := siteKey{entry.Rule, entry.Path}
		if used[key] < len(live[key]) {
			entry.Line = live[key][used[key]].Line
			used[key]++
		} else {
			stale = append(stale, entry)
		}
		refreshed = append(refreshed, entry)
	}
	for key, sites := range live {
		added = append(added, sites[used[key]:]...)
	}
	sortFindings(added)
	return stale, added, refreshed
}
