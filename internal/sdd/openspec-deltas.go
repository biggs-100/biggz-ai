package sdd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// largeMutationThreshold ports lib/openspec-deltas.ts: MODIFIED exceeding this
// line count is considered destructive and requires explicit approval.
const largeMutationThreshold = 20

// DeltaKind identifies the delta operation.
type DeltaKind string

const (
	DeltaAdded    DeltaKind = "ADDED"
	DeltaModified DeltaKind = "MODIFIED"
	DeltaRemoved  DeltaKind = "REMOVED"
)

// RequirementDelta is one requirement-level delta operation.
type RequirementDelta struct {
	Kind DeltaKind
	Name string
	Body string
}

// ParseResult is the output of ParseDeltaSpec.
type ParseResult struct {
	Deltas       []RequirementDelta
	HasRenamed   bool
	IsLegacyFlat bool
}

var (
	deltaSectionRe      = regexp.MustCompile(`(?m)^##\s+(ADDED|MODIFIED|REMOVED|RENAMED)\b`)
	deltaSectionExactRe = regexp.MustCompile(`(?m)^##\s+(ADDED|MODIFIED|REMOVED|RENAMED)(?:\s+Requirements)?\s*$`)
	requirementHeadingRe = regexp.MustCompile(`(?m)^###\s+Requirement:\s+(.+?)\s*$`)
	requirementAltRe     = regexp.MustCompile(`(?m)^###\s+REQ-[0-9]+:\s+\S`)
)

// ParseDeltaSpec parses a delta spec markdown document into requirement deltas.
// It detects ## RENAMED presence and legacy-flat (no requirement headings).
func ParseDeltaSpec(delta string) (ParseResult, error) {
	var res ParseResult
	// Detect RENAMED (case-sensitive as in spec, but also handle lower).
	if strings.Contains(delta, "## RENAMED") {
		res.HasRenamed = true
	}
	// Detect legacy flat: has content but no Requirement heading and no delta sections
	hasReq := requirementHeadingRe.MatchString(delta) || requirementAltRe.MatchString(delta)
	hasDeltaSection := deltaSectionRe.MatchString(delta)
	trimmed := strings.TrimSpace(delta)
	if trimmed != "" && !hasReq && !hasDeltaSection {
		// Could be flat spec with headings but not Requirement: treat as legacy flat if non-empty and no Requirement.
		// Also if file has no delta sections but has content, it's considered legacy flat for safety.
		// Only mark IsLegacyFlat when file looks like a spec without Requirement markers.
		if strings.Contains(delta, "#") {
			res.IsLegacyFlat = true
		}
	}
	// Also if has delta sections but no Requirement headings inside, still not legacy flat for delta;
	// legacy flat is mainly for main spec, but we expose it if delta has no requirements at all and HasRenamed false.
	if !hasReq && trimmed != "" && !res.HasRenamed {
		// If there are no deltas and HasRenamed false, but file is non-empty, treat as legacy flat when caller expects hint.
		// We already set above for flat without delta sections; for delta with empty requirement list we keep false to avoid false positives.
		// Keep as is.
	}

	lines := strings.Split(delta, "\n")
	var currentKind DeltaKind
	var currentName string
	var currentBodyLines []string

	flush := func() {
		if currentName != "" && currentKind != "" {
			body := strings.Join(currentBodyLines, "\n")
			body = strings.TrimSpace(body)
			// Ensure body starts with heading if not already? currentBodyLines already includes heading line
			res.Deltas = append(res.Deltas, RequirementDelta{
				Kind: currentKind,
				Name: strings.TrimSpace(currentName),
				Body: body,
			})
		}
		currentName = ""
		currentBodyLines = nil
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trimmedLine := strings.TrimSpace(line)
		// Check for delta section heading
		if m := deltaSectionExactRe.FindStringSubmatch(line); len(m) == 2 {
			// flush previous requirement before switching section
			flush()
			kindStr := strings.ToUpper(strings.TrimSpace(m[1]))
			switch kindStr {
			case "ADDED":
				currentKind = DeltaAdded
			case "MODIFIED":
				currentKind = DeltaModified
			case "REMOVED":
				currentKind = DeltaRemoved
			case "RENAMED":
				currentKind = ""
				res.HasRenamed = true
			}
			continue
		}
		// Also handle generic deltaSectionRe without anchored exact (fallback)
		// Already covered by exact; keep generic for lines like "## ADDED Requirements" already matched.
		// Check for requirement heading
		if m := requirementHeadingRe.FindStringSubmatch(line); len(m) == 2 {
			// New requirement starts
			flush()
			if currentKind == "" {
				// Requirement outside a delta section: ignore (delta specs must use ## ADDED/MODIFIED/REMOVED sections)
				currentName = ""
				currentBodyLines = nil
				continue
			}
			currentName = strings.TrimSpace(m[1])
			currentBodyLines = []string{line}
			continue
		}
		if currentName != "" && currentKind != "" {
			// Check if line starts a new top-level section (## ) which would have been handled above,
			// but if not matched exact, still treat as section boundary
			if strings.HasPrefix(trimmedLine, "## ") {
				// New section heading that is not ADDED/MODIFIED/REMOVED/RENAMED - could be new domain?
				// Flush current requirement and reset kind?
				flush()
				// Try to parse this heading as delta section again (generic)
				if gm := deltaSectionRe.FindStringSubmatch(line); len(gm) == 2 {
					kindStr := strings.ToUpper(gm[1])
					switch kindStr {
					case "ADDED":
						currentKind = DeltaAdded
					case "MODIFIED":
						currentKind = DeltaModified
					case "REMOVED":
						currentKind = DeltaRemoved
					case "RENAMED":
						currentKind = ""
						res.HasRenamed = true
					default:
						currentKind = ""
					}
				} else {
					currentKind = ""
				}
				continue
			}
			// Check for next requirement heading on same level already handled,
			// otherwise append to body
			currentBodyLines = append(currentBodyLines, line)
		}
	}
	flush()
	return res, nil
}

// ApplyDeltas applies requirement deltas to a main spec content.
// ADDED appends, MODIFIED replaces the matching requirement block, REMOVED deletes it.
// The function preserves header (content before first requirement) and ordering.
func ApplyDeltas(main string, deltas []RequirementDelta) (string, error) {
	if strings.TrimSpace(main) == "" && len(deltas) == 0 {
		return main, nil
	}
	// Parse main into header + ordered blocks
	header, order, blocks := parseMainSpec(main)
	// blocks is map[name]blockContent
	for _, d := range deltas {
		name := strings.TrimSpace(d.Name)
		switch d.Kind {
		case DeltaAdded:
			if _, exists := blocks[name]; exists {
				// Idempotent: if already exists with same body, skip
				if strings.TrimSpace(blocks[name]) == strings.TrimSpace(d.Body) {
					continue
				}
				// If exists with different body, treat as modify (to keep idempotent sync)
				blocks[name] = d.Body
				continue
			}
			blocks[name] = d.Body
			order = append(order, name)
		case DeltaModified:
			if _, exists := blocks[name]; !exists {
				return "", fmt.Errorf("MODIFIED requirement %q not found in main spec", name)
			}
			blocks[name] = d.Body
		case DeltaRemoved:
			if _, exists := blocks[name]; !exists {
				// Idempotent delete
				continue
			}
			delete(blocks, name)
			// remove from order
			newOrder := order[:0]
			for _, n := range order {
				if n != name {
					newOrder = append(newOrder, n)
				}
			}
			order = newOrder
		default:
			return "", fmt.Errorf("unknown delta kind %q", d.Kind)
		}
	}
	// Reconstruct
	var sb strings.Builder
	if header != "" {
		sb.WriteString(strings.TrimRight(header, "\n"))
		sb.WriteString("\n\n")
	}
	for i, name := range order {
		b, ok := blocks[name]
		if !ok {
			continue
		}
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		sb.WriteString(b)
		if i < len(order)-1 {
			sb.WriteString("\n\n")
		} else {
			sb.WriteString("\n")
		}
	}
	result := sb.String()
	// If header was empty and no blocks, return as is trimmed?
	if strings.TrimSpace(result) == "" {
		return "", nil
	}
	return result, nil
}

func parseMainSpec(main string) (header string, order []string, blocks map[string]string) {
	blocks = map[string]string{}
	if strings.TrimSpace(main) == "" {
		return "", nil, blocks
	}
	lines := strings.Split(main, "\n")
	// Find requirement heading indices
	type reqPos struct {
		name string
		line int
	}
	var positions []reqPos
	for i, line := range lines {
		if m := requirementHeadingRe.FindStringSubmatch(line); len(m) == 2 {
			positions = append(positions, reqPos{name: strings.TrimSpace(m[1]), line: i})
		}
	}
	if len(positions) == 0 {
		// No requirements: entire file is header
		return main, nil, blocks
	}
	firstReqLine := positions[0].line
	header = strings.Join(lines[:firstReqLine], "\n")
	// Extract blocks
	for idx, pos := range positions {
		start := pos.line
		var end int
		if idx+1 < len(positions) {
			end = positions[idx+1].line
		} else {
			end = len(lines)
		}
		blockLines := lines[start:end]
		// Trim trailing empty lines at block end that belong to separator
		for len(blockLines) > 0 && strings.TrimSpace(blockLines[len(blockLines)-1]) == "" {
			blockLines = blockLines[:len(blockLines)-1]
		}
		block := strings.Join(blockLines, "\n")
		blocks[pos.name] = block
		order = append(order, pos.name)
	}
	return header, order, blocks
}

// isLegacyFlat reports whether content is a legacy flat spec without Requirement headings.
func isLegacyFlat(content string) bool {
	if strings.TrimSpace(content) == "" {
		return false
	}
	if requirementHeadingRe.MatchString(content) || requirementAltRe.MatchString(content) {
		return false
	}
	// Has content but no requirement heading → legacy flat.
	return true
}

// isLargeModification reports whether a MODIFIED delta is considered large.
func isLargeModification(oldBody, newBody string) bool {
	newLines := len(strings.Split(strings.TrimSpace(newBody), "\n"))
	if newLines > largeMutationThreshold {
		return true
	}
	oldLines := len(strings.Split(strings.TrimSpace(oldBody), "\n"))
	diff := newLines - oldLines
	if diff < 0 {
		diff = -diff
	}
	if diff > largeMutationThreshold {
		return true
	}
	// Also check if newBody length is >50% larger than oldBody
	if oldLines > 0 && newLines > oldLines*3/2 && newLines-oldLines > 10 {
		return true
	}
	return false
}

// hasSyncDeltas reports whether changeRoot has any delta specs (files containing ADDED/MODIFIED/REMOVED sections).
func hasSyncDeltas(changeRoot string) bool {
	specsRoot := filepath.Join(changeRoot, "specs")
	files := findSpecFiles(specsRoot)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if deltaSectionRe.MatchString(string(data)) {
			return true
		}
		pr, err := ParseDeltaSpec(string(data))
		if err == nil && len(pr.Deltas) > 0 {
			return true
		}
	}
	return false
}

// HasSyncDeltas is exported for tests.
func HasSyncDeltas(changeRoot string) bool { return hasSyncDeltas(changeRoot) }

// detectCollision checks if another active change touches the same domain.
func detectCollision(change, workspaceRoot, domain string) (bool, string) {
	changesDir := filepath.Join(workspaceRoot, "openspec", "changes")
	entries, err := os.ReadDir(changesDir)
	if err != nil {
		return false, ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "archive" || name == change {
			continue
		}
		otherSpec := filepath.Join(changesDir, name, "specs", domain, "spec.md")
		if fileExists(otherSpec) {
			// Check if file has content (not empty)
			if hasContent(otherSpec) {
				return true, name
			}
		}
		// Also check any spec file under other change's specs directory that matches domain via walk
		otherDomainRoot := filepath.Join(changesDir, name, "specs", domain)
		if info, err := os.Stat(otherDomainRoot); err == nil && info.IsDir() {
			files := findSpecFiles(otherDomainRoot)
			if len(files) > 0 {
				return true, name
			}
		}
	}
	return false, ""
}

// DetectCollision is exported for tests.
func DetectCollision(change, workspaceRoot, domain string) (bool, string) {
	return detectCollision(change, workspaceRoot, domain)
}

// domainsWithDeltas returns the set of domains that have delta specs for the change.
func domainsWithDeltas(changeRoot string) []string {
	files := findSpecFiles(filepath.Join(changeRoot, "specs"))
	domains := map[string]bool{}
	for _, f := range files {
		dir := filepath.Dir(f)
		domain := filepath.Base(dir)
		if domain == "specs" || domain == "." {
			continue
		}
		domains[domain] = true
	}
	var out []string
	for d := range domains {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}
