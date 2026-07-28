package gitdiff

import (
	"testing"
)

func TestNormalizeLogicalPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"src/main.go", "src/main.go"},
		{"./src/main.go", "src/main.go"},
		{"src\\main.go", "src/main.go"},
		{"./src//deep/file.go", "src/deep/file.go"},
		{"", ""},
	}

	for _, tc := range tests {
		got := NormalizeLogicalPath(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeLogicalPath(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestIsGeneratedGoldenPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"src/main.go", false},
		{"src/main_test.go", true},
		{"src/testdata/fixture.json", true},
		{"src/golden/output.txt", true},
		{"src/mocks/mock_service.go", true},
		{"README.md", false},
	}

	for _, tc := range tests {
		got := IsGeneratedGoldenPath(tc.path)
		if got != tc.expected {
			t.Errorf("IsGeneratedGoldenPath(%q) = %v, want %v", tc.path, got, tc.expected)
		}
	}
}

func TestIsServiceTokenReviewPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"src/main.go", false},
		{"config/secrets.yaml", true},
		{".env.production", true},
		{"auth/tokens.go", true},
		{"certs/server.pem", true},
		{"README.md", false},
	}

	for _, tc := range tests {
		got := IsServiceTokenReviewPath(tc.path)
		if got != tc.expected {
			t.Errorf("IsServiceTokenReviewPath(%q) = %v, want %v", tc.path, got, tc.expected)
		}
	}
}
