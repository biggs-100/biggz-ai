package review

import (
	"path"
	"strings"
)

func isPassiveDocAllowed(p string) bool {
	if !isPassiveDocumentExtension(p) {
		return true
	}
	return isPassiveContentFile(p)
}

func isPassiveExtensionAllowed(p string) bool {
	if !isPassiveDocumentExtension(p) {
		return true
	}
	return isPassiveContentFile(p)
}

func isNonInertExecutablePath(p string) bool {
	lower := strings.ToLower(p)
	if _, ok := semanticSourceExtensions[path.Ext(lower)]; ok {
		return true
	}
	if _, ok := configurationExtensions[path.Ext(lower)]; ok {
		return true
	}
	if _, ok := configurationBasenames[lower]; ok {
		return true
	}
	if executionConfigPath([]string{p}) != "" {
		return true
	}
	return false
}

func hasValidInertLines(p string, diffSummary map[string]int) bool {
	if diffSummary == nil {
		return true
	}
	lines, present := diffSummary[p]
	return present && lines > 0
}

func isPathTriviallyInert(p string, diffSummary map[string]int) bool {
	if isDocumentationPath(p) {
		return isPassiveDocAllowed(p)
	}
	if isNonInertExecutablePath(p) {
		return false
	}
	if !isPassiveExtensionAllowed(p) {
		return false
	}
	return hasValidInertLines(p, diffSummary)
}
