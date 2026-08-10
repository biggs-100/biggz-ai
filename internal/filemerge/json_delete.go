package filemerge

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RemoveKeysJSONC removes the given dotted key paths from a JSONC document
// without touching any other byte: comments, whitespace, key order and
// sibling values are preserved. A path is a sequence of object keys from the
// document root, e.g. "agent.biggz-orchestrator". Paths that do not exist
// are no-ops. The document must parse as JSONC, otherwise an error is
// returned and data is left untouched.
//
// This is the surgical deletion counterpart of MergeJSONC: merge adds keys,
// RemoveKeysJSONC deletes exactly the keys owned by the caller. The deletion
// is textual (scanner-based), so untouched regions keep their exact bytes.
func RemoveKeysJSONC(data []byte, paths ...string) ([]byte, error) {
	if len(paths) == 0 {
		return data, nil
	}

	var check map[string]any
	if err := json.Unmarshal(stripComments(stripTrailingCommas(data)), &check); err != nil {
		return nil, fmt.Errorf("remove keys: document is not valid JSONC: %w", err)
	}

	result := data
	for _, p := range paths {
		result = removeKeyPath(result, strings.Split(p, "."))
	}

	// Re-validate the result so a scanner bug can never persist invalid JSON.
	if err := json.Unmarshal(stripComments(stripTrailingCommas(result)), &check); err != nil {
		return nil, fmt.Errorf("remove keys: result is not valid JSONC: %w", err)
	}
	return result, nil
}

// removeKeyPath removes one dotted path from a JSONC document. When the path
// has multiple segments, the search for the child key is restricted to the
// parent value's byte range so sibling keys with the same name are safe.
func removeKeyPath(data []byte, keys []string) []byte {
	if len(keys) == 0 {
		return data
	}
	start, end := findKeyRange(data, 0, len(data), keys[0], 1)
	if start < 0 {
		return data
	}
	if len(keys) == 1 {
		return removePairRange(data, start, end)
	}
	inner := removeKeyPath(data[start:end], keys[1:])
	return append(data[:start], append(inner, data[end:]...)...)
}

// removePairRange removes data[start:end] — the pair `"key": value` — plus
// the JSON punctuation that would otherwise be left dangling (one adjacent
// comma), leaving the rest of the document byte-identical.
func removePairRange(data []byte, start, end int) []byte {
	// Pair followed by a comma: drop the pair, the whitespace run before it
	// and the comma (keep the whitespace after the comma, which separates
	// the remaining sibling).
	i := end
	for i < len(data) && isJSONSpace(data[i]) {
		i++
	}
	if i < len(data) && data[i] == ',' {
		trim := start
		for trim > 0 && isJSONSpace(data[trim-1]) {
			trim--
		}
		return append(data[:trim], data[i+1:]...)
	}
	// Pair was the last in its container: drop the preceding comma and the
	// whitespace between it and the pair.
	i = start - 1
	for i >= 0 && isJSONSpace(data[i]) {
		i--
	}
	if i >= 0 && data[i] == ',' {
		return append(data[:i], data[start:]...)
	}
	return append(data[:start], data[end:]...)
}

// findKeyRange returns the byte range of the pair `"key": value` for the
// first occurrence of key at the given container depth (document-root keys
// sit at depth 1) inside data[lo:hi]. Returns (-1, -1) when not found.
//
// The scan is JSONC-aware: single/multi-line comments are skipped and keys
// inside string literals or comments never match. Object keys are
// distinguished from array elements by tracking the container type at each
// depth.
func findKeyRange(data []byte, lo, hi int, key string, wantDepth int) (int, int) {
	depth := 0
	var containers []bool // top of stack: true = object, false = array
	inString := false
	escaped := false
	inLineComment := false
	inBlockComment := false
	pendingKey := false

	i := lo
	for i < hi {
		c := data[i]

		if inLineComment {
			if c == '\n' {
				inLineComment = false
			}
			i++
			continue
		}
		if inBlockComment {
			if c == '*' && i+1 < hi && data[i+1] == '/' {
				inBlockComment = false
				i += 2
			} else {
				i++
			}
			continue
		}
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			i++
			continue
		}

		switch c {
		case '"':
			j := i + 1
			esc := false
			for j < hi {
				if esc {
					esc = false
				} else if data[j] == '\\' {
					esc = true
				} else if data[j] == '"' {
					break
				}
				j++
			}
			content := string(data[i+1 : j])
			if pendingKey && depth == wantDepth && content == key {
				if _, vend, ok := findValueRange(data, j+1, hi); ok {
					return i, vend
				}
			}
			pendingKey = false
			i = j + 1
			continue

		case '{':
			depth++
			containers = append(containers, true)
			pendingKey = true
		case '[':
			depth++
			containers = append(containers, false)
			pendingKey = false
		case '}':
			if depth > 0 {
				depth--
				if len(containers) > 0 {
					containers = containers[:len(containers)-1]
				}
			}
			pendingKey = false
		case ']':
			if depth > 0 {
				depth--
				if len(containers) > 0 {
					containers = containers[:len(containers)-1]
				}
			}
			pendingKey = false
		case ',':
			// Only object separators introduce a new key.
			pendingKey = len(containers) > 0 && containers[len(containers)-1]
		case ':':
			pendingKey = false
		case '/':
			if i+1 < hi && data[i+1] == '/' {
				inLineComment = true
				i++
			} else if i+1 < hi && data[i+1] == '*' {
				inBlockComment = true
				i++
			}
		}
		i++
	}
	return -1, -1
}

// findValueRange locates the value token starting at index start (the byte
// just past the key's closing quote, already past any ':' and whitespace by
// convention of the caller) and returns its byte range. Values may be
// strings, numbers, booleans, null, or nested objects/arrays; nested
// containers are balanced so comments and strings inside them are skipped.
func findValueRange(data []byte, start, hi int) (int, int, bool) {
	// Skip whitespace and any ':' separator.
	k := start
	for k < hi && isJSONSpace(data[k]) {
		k++
	}
	if k < hi && data[k] == ':' {
		k++
		for k < hi && isJSONSpace(data[k]) {
			k++
		}
	}
	if k >= hi {
		return 0, 0, false
	}

	switch data[k] {
	case '{', '[':
		depth := 0
		inStr := false
		esc := false
		inLine := false
		inBlock := false
		j := k
		for j < hi {
			c := data[j]
			if inLine {
				if c == '\n' {
					inLine = false
				}
				j++
				continue
			}
			if inBlock {
				if c == '*' && j+1 < hi && data[j+1] == '/' {
					inBlock = false
					j++
				}
				j++
				continue
			}
			if inStr {
				if esc {
					esc = false
				} else if c == '\\' {
					esc = true
				} else if c == '"' {
					inStr = false
				}
				j++
				continue
			}
			if c == '"' {
				inStr = true
			} else if c == '/' && j+1 < hi && data[j+1] == '/' {
				inLine = true
				j++
			} else if c == '/' && j+1 < hi && data[j+1] == '*' {
				inBlock = true
				j++
			} else if c == '{' || c == '[' {
				depth++
			} else if c == '}' || c == ']' {
				depth--
				if depth == 0 {
					return k, j + 1, true
				}
			}
			j++
		}
		return 0, 0, false
	case '"':
		j := k + 1
		esc := false
		for j < hi {
			if esc {
				esc = false
			} else if data[j] == '\\' {
				esc = true
			} else if data[j] == '"' {
				return k, j + 1, true
			}
			j++
		}
		return 0, 0, false
	default:
		// Scalar: number, true, false, null.
		j := k
		for j < hi && !isJSONSpace(data[j]) && data[j] != ',' && data[j] != '}' && data[j] != ']' {
			j++
		}
		return k, j, true
	}
}

func isJSONSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
