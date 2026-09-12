package sdd

import (
	"errors"
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
		if err := applyModifiedDelta(blocks, order, name, d.Body); err != nil {
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

// modifiedNotFoundHint is the actionable tail shared by every
// MODIFIED-requirement-not-found error. Exact-name matching stays
// authoritative: the hint only explains how to express a retitle.
const modifiedNotFoundHint = "the delta heading must match the main spec heading verbatim — a retitle must be expressed as REMOVED + ADDED, or the exact existing heading kept"

// maxModifiedHintRunes bounds the heading text quoted back in the
// MODIFIED-not-found error so the message stays stable for very long titles.
const maxModifiedHintRunes = 120

// requirementIDPrefixRe captures the leading REQ-… id of a requirement name,
// e.g. "REQ-1" in "REQ-1 — Engram Import Dispatch" and "REQ-PIPELINE-001" in
// "REQ-PIPELINE-001 — StagePlan Prepare/Apply Contract".
var requirementIDPrefixRe = regexp.MustCompile(`(?i)^REQ-[A-Za-z0-9]+(?:-[A-Za-z0-9]+)*`)

// requirementIDPrefix returns the uppercased leading REQ-… id, or "" when the
// name does not start with one.
func requirementIDPrefix(name string) string {
	m := requirementIDPrefixRe.FindString(strings.TrimSpace(name))
	return strings.ToUpper(m)
}

// normalizeRequirementName lowercases and collapses whitespace so two names
// that differ only in case or spacing compare equal.
func normalizeRequirementName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

// modifiedCandidateName returns the first main-spec requirement (in spec
// order) that shares the delta name's REQ-… id prefix or has an identical
// normalized name. Exact matching remains authoritative; this only feeds the
// error hint, so there is no fuzzy matching and no behavior change.
func modifiedCandidateName(order []string, blocks map[string]string, name string) string {
	nameID := requirementIDPrefix(name)
	nameNorm := normalizeRequirementName(name)
	for _, cand := range order {
		if _, ok := blocks[cand]; !ok {
			continue
		}
		if candID := requirementIDPrefix(cand); candID != "" && nameID != "" && candID == nameID {
			return cand
		}
		if normalizeRequirementName(cand) == nameNorm {
			return cand
		}
	}
	return ""
}

// boundHeadingForError truncates a heading to maxModifiedHintRunes so the
// not-found error stays bounded for pathologically long titles.
func boundHeadingForError(name string) string {
	runes := []rune(name)
	if len(runes) <= maxModifiedHintRunes {
		return name
	}
	return string(runes[:maxModifiedHintRunes]) + "…"
}

func applyModifiedDelta(blocks map[string]string, order []string, name, body string) error {
	if _, exists := blocks[name]; !exists {
		msg := fmt.Sprintf("MODIFIED requirement %q not found in main spec; %s", boundHeadingForError(name), modifiedNotFoundHint)
		if cand := modifiedCandidateName(order, blocks, name); cand != "" {
			msg += fmt.Sprintf("; main spec has a similar requirement heading: %q", boundHeadingForError(cand))
		}
		return errors.New(msg)
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
