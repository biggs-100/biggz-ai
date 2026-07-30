package filemerge

import (
	"encoding/json"
	"fmt"
)

// MergeJSONC strips JSONC non-standard syntax (comments, trailing commas) from
// both existing and overlay, then deep-merges overlay into existing. Nested maps
// are merged recursively. Arrays in overlay replace existing arrays entirely. If
// a value in overlay contains "__replace__": true, the target value is replaced
// entirely (the __replace__ key itself is stripped from output).
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

	deepMerge(base, patch)

	return json.MarshalIndent(base, "", "  ")
}

// deepMerge recursively merges overlay into base. Both must be non-nil maps.
// Arrays in overlay replace existing arrays. Nested maps merge recursively.
// If an overlay map contains "__replace__": true, the target is replaced entirely
// and the __replace__ key is stripped from the output.
func deepMerge(base, overlay map[string]any) {
	for k, v := range overlay {
		// Check for __replace__ sentinel: force-replace the entire target
		if overlayMap, ok := v.(map[string]any); ok {
			if replace, ok := overlayMap["__replace__"]; ok {
				if replaceBool, ok := replace.(bool); ok && replaceBool {
					delete(overlayMap, "__replace__")
					base[k] = overlayMap
					continue
				}
			}
		}

		// If both values are maps, recurse
		baseVal, baseIsMap := base[k].(map[string]any)
		overlayVal, overlayIsMap := v.(map[string]any)
		if baseIsMap && overlayIsMap {
			deepMerge(baseVal, overlayVal)
		} else {
			// Otherwise overlay wins (replace flat value or array)
			base[k] = v
		}
	}
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
