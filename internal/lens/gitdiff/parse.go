package gitdiff

import (
	"regexp"
	"strconv"
	"strings"
)

// diffStatRegex matches git diff --stat output lines.
// Format: path | N +additions -deletions
// Example: "main.go | 10 ++++++++---"
var diffStatRegex = regexp.MustCompile(`^(.+?)\s*\|\s*(\d+)\s*(\++)(\-*)\s*$`)

// ParseDiffStat parses the output of git diff --stat into a slice of DiffFile.
// It is a pure function with no side effects, making it directly testable.
//
// Input format (one line per file):
//
//	path/to/file.go | 42 ++++++++++----------
//	README.md | 2 +-
func ParseDiffStat(output string) []DiffFile {
	var files []DiffFile
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches := diffStatRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		path := strings.TrimSpace(matches[1])
		total, _ := strconv.Atoi(matches[2])
		plusCount := len(matches[3])
		minusCount := len(matches[4])

		additions, deletions := total, 0
		if plusCount+minusCount > 0 {
			additions = total * plusCount / (plusCount + minusCount)
			deletions = total - additions
		}

		files = append(files, DiffFile{
			Path:      path,
			Additions: additions,
			Deletions: deletions,
		})
	}
	return files
}

// rawDiffModeRegex matches lines in git diff --raw output that indicate
// a mode change to executable (100755). Format:
//
//	:100644 100755 abc123... def456... M	path/to/file.sh
var rawDiffModeRegex = regexp.MustCompile(`^:\d+\s+100755\s`)

// DetectModeChanges checks git diff --raw output for executable mode
// transitions (new mode is 100755). It is a pure function.
func DetectModeChanges(rawOutput string) bool {
	for _, line := range strings.Split(rawOutput, "\n") {
		if rawDiffModeRegex.MatchString(line) {
			return true
		}
	}
	return false
}
