package filemerge

import (
	"fmt"
	"strings"
)

// sectionMarker returns opening and closing markers for a named section.
func sectionMarker(name string) (open, close string) {
	return fmt.Sprintf("<!-- section:%s -->", name),
		fmt.Sprintf("<!-- /section -->")
}

// InjectSection appends a new marker-delimited section to content. The section
// is delimited by <!-- section:name --> and <!-- /section --> markers.
// If content is empty, it creates the section as the sole content.
func InjectSection(content string, sectionName string, newSection []byte) ([]byte, error) {
	open, close := sectionMarker(sectionName)
	section := open + "\n" + string(newSection) + "\n" + close

	if len(content) == 0 {
		return []byte(section), nil
	}

	// Ensure trailing newline before appending
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return []byte(content + section + "\n"), nil
}

// ReplaceSection finds an existing marker-delimited section by name and
// replaces the content between the markers, preserving the markers themselves.
// Returns an error if the section markers are not found.
func ReplaceSection(content string, sectionName string, newSection []byte) ([]byte, error) {
	open, close := sectionMarker(sectionName)

	openIdx := strings.Index(content, open)
	if openIdx < 0 {
		return nil, fmt.Errorf("section %q not found: opening marker %q missing", sectionName, open)
	}

	closeIdx := strings.Index(content[openIdx:], close)
	if closeIdx < 0 {
		return nil, fmt.Errorf("section %q not found: closing marker %q missing", sectionName, close)
	}
	closeIdx += openIdx

	// Content between markers starts after the opening marker and newline
	afterOpen := openIdx + len(open)
	if afterOpen < len(content) && content[afterOpen] == '\n' {
		afterOpen++
	}

	// Build result: everything before the opening marker + opening marker +
	// new content + closing marker + everything after closing marker
	var b strings.Builder
	b.Grow(len(content) + len(newSection))

	// Before the opening marker
	b.WriteString(content[:openIdx])

	// Opening marker + newline + new content + newline + closing marker
	b.WriteString(open)
	b.WriteString("\n")
	b.Write(newSection)
	b.WriteString("\n")
	b.WriteString(close)

	// After the closing marker
	tail := closeIdx + len(close)
	if tail < len(content) {
		b.WriteString(content[tail:])
	}

	return []byte(b.String()), nil
}
