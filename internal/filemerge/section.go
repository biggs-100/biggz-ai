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

// ExtractHTMLCommentSection extracts the content between a paired
// <!-- section:NAME --> ... <!-- /section:NAME --> marker pair. A missing,
// lone, or reversed marker pair returns the full content unchanged.
func ExtractHTMLCommentSection(content, name string) string {
	openMarker := "<!-- section:" + name + " -->"
	closeMarker := "<!-- /section:" + name + " -->"
	start := strings.Index(content, openMarker)
	end := strings.Index(content, closeMarker)
	if start == -1 || end == -1 || end <= start {
		return content
	}
	afterOpen := start + len(openMarker)
	return strings.TrimLeft(content[afterOpen:end], " \t\r\n")
}

const (
	markerPrefix = "<!-- biggz:"
	markerSuffix = " -->"
	closePrefix  = "<!-- /biggz:"
)

// openMarker returns the opening marker for a section ID.
func openMarker(sectionID string) string {
	return markerPrefix + sectionID + markerSuffix
}

// closeMarker returns the closing marker for a section ID.
func closeMarker(sectionID string) string {
	return closePrefix + sectionID + markerSuffix
}

// stripOrphanMarkers removes unpaired opening or closing markers for the given
// sectionID from content before injection logic runs.
func stripOrphanMarkers(content, open, close string) string {
	for {
		openIdx := strings.Index(content, open)
		closeIdx := strings.Index(content, close)

		switch {
		case openIdx < 0 && closeIdx < 0:
			return content
		case openIdx < 0 && closeIdx >= 0:
			content = content[:closeIdx] + content[closeIdx+len(close):]
		case openIdx >= 0 && closeIdx < 0:
			content = content[:openIdx] + content[openIdx+len(open):]
		case closeIdx < openIdx:
			content = content[:closeIdx] + content[closeIdx+len(close):]
		default:
			return content
		}
	}
}

// InjectMarkdownSection replaces or appends a marked section in a markdown file.
// Markers use HTML comments: <!-- biggz:SECTION_ID --> ... <!-- /biggz:SECTION_ID -->
// If the section already exists, its content is replaced.
// If it doesn't exist, it's appended at the end.
// Content outside markers is never touched.
// If content is empty, the section (including markers) is removed.
func InjectMarkdownSection(existing, sectionID, content string) string {
	open := openMarker(sectionID)
	close := closeMarker(sectionID)

	existing = stripOrphanMarkers(existing, open, close)

	openIdx := strings.Index(existing, open)
	closeIdx := strings.Index(existing, close)

	if openIdx >= 0 && closeIdx >= 0 && closeIdx > openIdx {
		before := existing[:openIdx]
		after := existing[closeIdx+len(close):]

		var preservedAfter strings.Builder
		for {
			duplicateOpen := strings.Index(after, open)
			if duplicateOpen < 0 {
				preservedAfter.WriteString(after)
				break
			}
			bodyStart := duplicateOpen + len(open)
			duplicateCloseOffset := strings.Index(after[bodyStart:], close)
			if duplicateCloseOffset < 0 {
				preservedAfter.WriteString(after)
				break
			}
			duplicateEnd := bodyStart + duplicateCloseOffset + len(close)
			preservedAfter.WriteString(after[:duplicateOpen])
			after = after[duplicateEnd:]
		}
		after = preservedAfter.String()

		if content == "" {
			if len(after) > 0 && after[0] == '\n' {
				after = after[1:]
			}
			result := strings.TrimRight(before, "\n")
			if after != "" {
				if result != "" {
					result += "\n"
				}
				result += after
			} else if result != "" {
				result += "\n"
			}
			return result
		}

		var sb strings.Builder
		sb.WriteString(before)
		sb.WriteString(open)
		sb.WriteString("\n")
		sb.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString(close)
		sb.WriteString(after)
		return sb.String()
	}

	if content == "" {
		return existing
	}

	var sb strings.Builder
	sb.WriteString(existing)
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		sb.WriteString("\n")
	}
	if existing != "" {
		sb.WriteString("\n")
	}
	sb.WriteString(open)
	sb.WriteString("\n")
	sb.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString(close)
	sb.WriteString("\n")
	return sb.String()
}
