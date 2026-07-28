// Package gitdiff provides safe path handling for git operations.
// Ported from gentle-ai's path safety and secure_open modules.
package gitdiff

import (
	"path/filepath"
	"strings"
)

// NormalizeLogicalPath normalizes a file path for cross-platform comparison.
// Handles Windows backslashes, case sensitivity, and Unicode normalization.
func NormalizeLogicalPath(path string) string {
	// Convert to forward slashes
	path = filepath.ToSlash(path)

	// Remove leading ./ and .\
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, ".\\")

	// Collapse multiple slashes
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	return path
}

// IsGeneratedGoldenPath checks if a path is a generated/golden file.
func IsGeneratedGoldenPath(path string) bool {
	normalized := NormalizeLogicalPath(path)

	// Common generated file patterns
	generatedPatterns := []string{
		"/testdata/",
		"/golden/",
		"/fixtures/",
		"/mocks/",
		"/mock_",
		"_test.go",
		".generated.",
		"go.sum",
		"package-lock.json",
		"yarn.lock",
	}

	for _, pattern := range generatedPatterns {
		if strings.Contains(normalized, pattern) {
			return true
		}
	}
	return false
}

// IsServiceTokenReviewPath checks if a path might contain service tokens or secrets.
func IsServiceTokenReviewPath(path string) bool {
	normalized := NormalizeLogicalPath(path)
	segments := strings.Split(normalized, "/")

	sensitivePatterns := []string{
		"secret",
		"token",
		"credential",
		"password",
		".env",
		"key",
		"cert",
		".pem",
		".key",
	}

	for _, segment := range segments {
		lower := strings.ToLower(segment)
		for _, pattern := range sensitivePatterns {
			if strings.Contains(lower, pattern) {
				return true
			}
		}
	}
	return false
}
