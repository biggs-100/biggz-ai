package sdd

import (
	"fmt"
	"regexp"
	"strings"
)

// deltaParseState holds incremental parsing state for delta specs.
type deltaParseState struct {
	res              *ParseResult
	currentKind      DeltaKind
	currentName      string
	currentBodyLines []string
}

func newDeltaParseState(res *ParseResult) *deltaParseState {
	return &deltaParseState{res: res}
}

func (s *deltaParseState) flush() {
	if s.currentName == "" || s.currentKind == "" {
		s.currentName = ""
		s.currentBodyLines = nil
		return
	}
	body := strings.Join(s.currentBodyLines, "\n")
	body = strings.TrimSpace(body)
	s.res.Deltas = append(s.res.Deltas, RequirementDelta{
		Kind: s.currentKind,
		Name: strings.TrimSpace(s.currentName),
		Body: body,
	})
	s.currentName = ""
	s.currentBodyLines = nil
}

func (s *deltaParseState) applyDeltaKind(kindStr string) {
	switch kindStr {
	case "ADDED":
		s.currentKind = DeltaAdded
	case "MODIFIED":
		s.currentKind = DeltaModified
	case "REMOVED":
		s.currentKind = DeltaRemoved
	case "RENAMED":
		s.currentKind = ""
		s.res.HasRenamed = true
	}
}

// --- ParseDeltaSpec helpers (pure where possible) ---

func deltaHasRenamed(delta string) bool {
	return regexp.MustCompile(`(?m)^##\s+RENAMED\b`).MatchString(delta)
}

func deltaHasRequirement(delta string) bool {
	return requirementHeadingRe.MatchString(delta) || requirementAltRe.MatchString(delta)
}

func detectLegacyFlat(delta string) bool {
	trimmed := strings.TrimSpace(delta)
	if trimmed == "" {
		return false
	}
	if deltaHasRequirement(delta) {
		return false
	}
	if deltaSectionRe.MatchString(delta) {
		return false
	}
	return strings.Contains(delta, "#")
}

func tryHandleDeltaSection(line string, s *deltaParseState) bool {
	m := deltaSectionExactRe.FindStringSubmatch(line)
	if len(m) != 2 {
		return false
	}
	s.flush()
	kindStr := strings.ToUpper(strings.TrimSpace(m[1]))
	s.applyDeltaKind(kindStr)
	return true
}

func tryHandleRequirement(line string, s *deltaParseState) bool {
	m := requirementHeadingRe.FindStringSubmatch(line)
	if len(m) != 2 {
		return false
	}
	s.flush()
	if s.currentKind == "" {
		s.currentName = ""
		s.currentBodyLines = nil
		return true
	}
	s.currentName = strings.TrimSpace(m[1])
	s.currentBodyLines = []string{line}
	return true
}

func appendDeltaBody(line string, s *deltaParseState) {
	if s.currentName == "" || s.currentKind == "" {
		return
	}
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "## ") {
		s.flush()
		handleGenericSection(line, s)
		return
	}
	s.currentBodyLines = append(s.currentBodyLines, line)
}

func handleGenericSection(line string, s *deltaParseState) {
	gm := deltaSectionRe.FindStringSubmatch(line)
	if len(gm) != 2 {
		s.currentKind = ""
		return
	}
	kindStr := strings.ToUpper(gm[1])
	switch kindStr {
	case "ADDED":
		s.currentKind = DeltaAdded
	case "MODIFIED":
		s.currentKind = DeltaModified
	case "REMOVED":
		s.currentKind = DeltaRemoved
	case "RENAMED":
		s.currentKind = ""
		s.res.HasRenamed = true
	default:
		s.currentKind = ""
	}
}

// --- ApplyDeltas helpers (pure where possible) ---

func applyAllDeltas(blocks map[string]string, order []string, deltas []RequirementDelta) ([]string, error) {
	for _, d := range deltas {
		var err error
		order, err = applySingleDelta(blocks, order, d)
		if err != nil {
			return nil, err
		}
	}
	return order, nil
}

func applySingleDelta(blocks map[string]string, order []string, d RequirementDelta) ([]string, error) {
	name := strings.TrimSpace(d.Name)
	switch d.Kind {
	case DeltaAdded:
		return applyAddedDelta(blocks, order, name, d.Body), nil
	case DeltaModified:
		if err := applyModifiedDelta(blocks, name, d.Body); err != nil {
			return order, err
		}
		return order, nil
	case DeltaRemoved:
		return applyRemovedDelta(blocks, order, name), nil
	default:
		return order, fmt.Errorf("unknown delta kind %q", d.Kind)
	}
}

func applyAddedDelta(blocks map[string]string, order []string, name, body string) []string {
	if existing, exists := blocks[name]; exists {
		if strings.TrimSpace(existing) == strings.TrimSpace(body) {
			return order
		}
		blocks[name] = body
		return order
	}
	blocks[name] = body
	return append(order, name)
}

func applyModifiedDelta(blocks map[string]string, name, body string) error {
	if _, exists := blocks[name]; !exists {
		return fmt.Errorf("MODIFIED requirement %q not found in main spec", name)
	}
	blocks[name] = body
	return nil
}

func applyRemovedDelta(blocks map[string]string, order []string, name string) []string {
	if _, exists := blocks[name]; !exists {
		return order
	}
	delete(blocks, name)
	return removeFromOrder(order, name)
}

func removeFromOrder(order []string, name string) []string {
	newOrder := order[:0]
	for _, n := range order {
		if n != name {
			newOrder = append(newOrder, n)
		}
	}
	return newOrder
}

func rebuildSpec(header string, order []string, blocks map[string]string) string {
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
	return sb.String()
}
