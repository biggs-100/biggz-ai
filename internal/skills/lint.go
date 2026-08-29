package skills

import (
	"fmt"
	"os"
	"strings"
)

func LintSkill(path string) (int, []string, error) {
	d, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, err
	}
	body, fm, fmErr := extractFrontmatter(string(d))
	var diags []string
	if fmErr != "" {
		diags = append(diags, "FAIL: "+fmErr)
	} else if e := validateFrontmatter(fm); e != "" {
		diags = append(diags, "FAIL: "+e)
	}
	t := CountTokens(body)
	switch {
	case t > 1000:
		diags = append(diags, fmt.Sprintf("FAIL: token count %d exceeds hard limit 1000", t))
	case t > 450:
		diags = append(diags, fmt.Sprintf("WARN: token count %d exceeds ideal 450 (warn until 1000)", t))
	case t > 0 && t < 180:
		diags = append(diags, fmt.Sprintf("WARN: token count %d below ideal 180", t))
	}
	return t, diags, nil
}
func CountTokens(b string) int {
	if strings.TrimSpace(b) == "" {
		return 0
	}
	return len(strings.Fields(b))
}
func extractFrontmatter(c string) (body, fm string, errMsg string) {
	if !strings.HasPrefix(c, "---\n") && !strings.HasPrefix(c, "---\r\n") {
		return c, "", "missing frontmatter"
	}
	rest := c[4:]
	end := strings.Index(rest, "\n---")
	if end == -1 {
		end = strings.Index(rest, "\r\n---")
	}
	if end == -1 {
		return c, "", "missing frontmatter closing ---"
	}
	fm = rest[:end]
	body = strings.TrimLeft(rest[end+4:], "\r\n")
	return body, fm, ""
}
func validateFrontmatter(fm string) string {
	var dl string
	found := false
	for _, l := range strings.Split(fm, "\n") {
		trim := strings.TrimSpace(l)
		if !strings.HasPrefix(trim, "description:") {
			continue
		}
		found = true
		dl = trim
		val := strings.TrimSpace(strings.TrimPrefix(trim, "description:"))
		if val == "" {
			return "description missing value (multi-line not allowed)"
		}
		if !(strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) && !(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
			return "description must be single-line quoted"
		}
		inner := val[1 : len(val)-1]
		if len(inner) > 250 {
			return fmt.Sprintf("description exceeds 250 chars (%d)", len(inner))
		}
		if !strings.Contains(inner, "Trigger:") && !strings.Contains(inner, "trigger:") {
			return "description missing trigger keyword"
		}
		break
	}
	if !found || dl == "" {
		return "missing description in frontmatter"
	}
	return ""
}
func HasHardFailure(d []string) bool {
	for _, x := range d {
		if strings.HasPrefix(x, "FAIL:") {
			return true
		}
	}
	return false
}
func HasWarning(d []string) bool {
	for _, x := range d {
		if strings.HasPrefix(x, "WARN:") {
			return true
		}
	}
	return false
}
