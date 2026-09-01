package sdd

import "strings"

// Pure helpers for sddReadAllowedEntries — extracted to keep cyclomatic <15 and cognitive <20.
// Each helper is deliberately small and side-effect free.

func normalizeSurfaceLines(following string) []string {
	linesRaw := strings.Split(following, "\n")
	lines := make([]string, len(linesRaw))
	for i, l := range linesRaw {
		l = strings.TrimSuffix(l, "\r")
		lines[i] = strings.TrimSpace(l)
	}
	return lines
}

func sectionUntilNextHeading(lines []string) []string {
	for i, l := range lines {
		if sddHeadingRe.MatchString(l) {
			return lines[:i]
		}
	}
	return lines
}

func hasLaterSurfaceEntry(section []string, fromIdx int) bool {
	for _, c := range section[fromIdx+1:] {
		if sddLooksLikeSurfaceEntry(c) {
			return true
		}
	}
	return false
}

func collectAllSurfaceEntries(section []string) []string {
	var all []string
	for _, c := range section {
		if len(c) == 0 {
			continue
		}
		all = append(all, sddReadSurfaceEntry(c))
	}
	return all
}

func extractAllowedEntries(section []string) []string {
	var entries []string
	for idx, line := range section {
		if len(line) == 0 {
			continue
		}
		e := sddReadSurfaceEntry(line)
		if sddWhitespaceRe.MatchString(e) {
			if hasLaterSurfaceEntry(section, idx) {
				return collectAllSurfaceEntries(section)
			}
			break
		}
		entries = append(entries, e)
	}
	return entries
}

// Pure helpers for resolveNextRecommended — extracted to keep cyclomatic <15 and cognitive <20.

func nextForImmediateReady(dependencies Dependencies, applyState ApplyState, verifyReportDone bool, remediation RemediationState) string {
	if dependencies.Apply == DependencyReady {
		return "apply"
	}
	if dependencies.Verify == DependencyReady {
		return "verify"
	}
	if isRemediationNeeded(applyState, verifyReportDone, dependencies.Verify, remediation) {
		return "remediate"
	}
	if dependencies.Sync == DependencyReady {
		return "sync"
	}
	if isArchiveReady(dependencies, applyState) {
		return "archive"
	}
	return ""
}

func isRemediationNeeded(applyState ApplyState, verifyReportDone bool, verifyDep DependencyState, remediation RemediationState) bool {
	return applyState == ApplyAllDone && verifyReportDone && verifyDep != DependencyAllDone && remediation.Required
}

func isArchiveReady(dependencies Dependencies, applyState ApplyState) bool {
	return dependencies.Verify == DependencyAllDone && applyState == ApplyAllDone && (dependencies.Sync == DependencyAllDone || dependencies.Sync == "")
}

func nextForPlanning(dependencies Dependencies) string {
	if dependencies.Proposal != DependencyAllDone {
		return "propose"
	}
	if dependencies.Specs != DependencyAllDone {
		return "spec"
	}
	if dependencies.Design != DependencyAllDone {
		return "design"
	}
	if dependencies.Tasks != DependencyAllDone {
		return "tasks"
	}
	return ""
}
