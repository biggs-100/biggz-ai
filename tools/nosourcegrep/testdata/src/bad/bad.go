package bad

import (
	"os"
	"strings"
)

func Bad() {
	src, _ := os.ReadFile("internal/foo.go")  // want "source-grep: os.ReadFile on source file"
	if strings.Contains(string(src), "foo") { // want "source-grep: strings.Contains"
		println("found")
	}
	// also test expect(src).toContain style via string literal
	_ = "expect(src).toContain" // want "source-grep: expect"
}
