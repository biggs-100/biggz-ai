package filemerge

import (
	"encoding/json"
	"fmt"
)

// MergeJSONC strips JSONC non-standard syntax (comments, trailing commas) from
// both existing and overlay, then merges overlay's top-level keys into existing.
// Overlay keys replace existing ones; all other keys are preserved.
func MergeJSONC(existing, overlay []byte) ([]byte, error) {
	if len(overlay) == 0 {
		return existing, nil
	}

	existing = stripComments(stripTrailingCommas(existing))
	overlay = stripComments(stripTrailingCommas(overlay))

	var base, patch map[string]any
	if err := json.Unmarshal(existing, &base); err != nil {
		return nil, fmt.Errorf("merge: existing is not valid JSON: %w", err)
	}
	if err := json.Unmarshal(overlay, &patch); err != nil {
		return nil, fmt.Errorf("merge: overlay is not valid JSON: %w", err)
	}

	for k, v := range patch {
		base[k] = v
	}

	return json.MarshalIndent(base, "", "  ")
}

// stripComments removes // single-line and /* */ multi-line comments from JSONC data.
// It correctly preserves // and /* sequences inside JSON string literals.
func stripComments(data []byte) []byte {
	var out []byte
	inString := false

	for i := 0; i < len(data); i++ {
		ch := data[i]

		// Handle escape sequences inside strings
		if inString && ch == '\\' && i+1 < len(data) {
			out = append(out, ch, data[i+1])
			i++
			continue
		}

		// Toggle string state (handles JSON strings)
		if ch == '"' {
			inString = !inString
			out = append(out, ch)
			continue
		}

		if inString {
			out = append(out, ch)
			continue
		}

		// Check for comments
		if ch == '/' && i+1 < len(data) {
			next := data[i+1]

			// Single-line comment //
			if next == '/' {
				for i < len(data) && data[i] != '\n' {
					i++
				}
				out = append(out, '\n')
				continue
			}

			// Multi-line comment /* */
			if next == '*' {
				i += 2
				for i < len(data) {
					if data[i] == '*' && i+1 < len(data) && data[i+1] == '/' {
						i += 2
						break
					}
					i++
				}
				continue
			}
		}

		out = append(out, ch)
	}

	return out
}

// stripTrailingCommas removes trailing commas before } or ] characters.
func stripTrailingCommas(data []byte) []byte {
	var out []byte
	for i := 0; i < len(data); i++ {
		if data[i] == ',' {
			// Look ahead skipping whitespace to see if followed by } or ]
			j := i + 1
			for j < len(data) && (data[j] == ' ' || data[j] == '\t' || data[j] == '\n' || data[j] == '\r') {
				j++
			}
			if j < len(data) && (data[j] == '}' || data[j] == ']') {
				// Skip the trailing comma
				continue
			}
		}
		out = append(out, data[i])
	}
	return out
}
