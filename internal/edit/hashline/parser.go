package hashline

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Op is the directive operation.
type Op string

const (
	OpPUT Op = "PUT"
	OpCUT Op = "CUT"
)

// Directive is a parsed PUT/CUT line.
type Directive struct {
	Op      Op
	Start   int
	End     int
	HashTag string // 4-hex upper
	Raw     string
}

var directiveRe = regexp.MustCompile(`^(PUT|CUT)\s+(?:(\d+)\.=(\d+):|<\s*(\d+)(?::)?)\s+#([0-9a-fA-F]{4})\b`)

// Parse parses a single hashline directive. It requires a #A1B2 4-hex tag.
// Missing/malformed/non-hex tags are rejected. Whole-file fallback does not exist.
func Parse(line string) (Directive, error) {
	trimmed := strings.TrimSpace(line)
	m := directiveRe.FindStringSubmatch(trimmed)
	if m == nil {
		return Directive{}, fmt.Errorf("invalid hashline directive %q: expected PUT/CUT with #A1B2", line)
	}
	op := Op(m[1])
	hash := strings.ToUpper(m[5])
	var start, end int
	if m[2] != "" && m[3] != "" {
		var err error
		start, err = strconv.Atoi(m[2])
		if err != nil {
			return Directive{}, fmt.Errorf("invalid start %q", m[2])
		}
		end, err = strconv.Atoi(m[3])
		if err != nil {
			return Directive{}, fmt.Errorf("invalid end %q", m[3])
		}
		if start < 1 || end < 1 || start > end {
			return Directive{}, fmt.Errorf("invalid range %d.= %d", start, end)
		}
	} else if m[4] != "" {
		n, err := strconv.Atoi(m[4])
		if err != nil {
			return Directive{}, fmt.Errorf("invalid line %q", m[4])
		}
		if n < 1 {
			return Directive{}, fmt.Errorf("invalid line %d", n)
		}
		start = n
		end = n
	} else {
		return Directive{}, fmt.Errorf("invalid directive %q", line)
	}
	return Directive{Op: op, Start: start, End: end, HashTag: hash, Raw: trimmed}, nil
}

// ValidateSeen checks that the directive's range lies within the seen ranges
// captured by the read hook. Unseen ranges are rejected.
func ValidateSeen(d Directive, seen [][2]int) error {
	if len(seen) == 0 {
		return fmt.Errorf("unseen range %d.= %d: no seen ranges recorded", d.Start, d.End)
	}
	for _, r := range seen {
		if d.Start >= r[0] && d.End <= r[1] {
			return nil
		}
	}
	return fmt.Errorf("unseen range %d.= %d: not in seen %v", d.Start, d.End, seen)
}
