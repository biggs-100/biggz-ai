package hashline

import (
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
)

func TestParse(t *testing.T) {
	h := Hash4(filemerge.ComputeHash([]byte("hello\n")))
	tests := []struct {
		name string
		line string
		ok   bool
	}{
		{"PUT range", "PUT 1.=1: #" + h, true},
		{"CUT range", "CUT 2.=2: #" + h, true},
		{"PUT <N", "PUT <5 #" + h, true},
		{"PUT <N colon", "PUT <5: #" + h, true},
		{"CUT <N", "CUT <10 #" + h, true},
		{"missing tag", "PUT 10.=20:", false},
		{"non-hex", "PUT 10.=20: #ZZZZ", false},
		{"short hash", "PUT 10.=20: #A1B", false},
		{"missing colon", "PUT 10.=20 #A1B2", false},
		{"wrong op", "HELLO 10.=20: #A1B2", false},
		{"whole file", "PUT #A1B2", false},
	}
	for _, tc := range tests {
		_, err := Parse(tc.line)
		if tc.ok && err != nil {
			t.Errorf("%s should pass: %v", tc.name, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s should fail", tc.name)
		}
	}
	// case insensitive
	d, _ := Parse("PUT 1.=1: #a1b2")
	if d.HashTag != "A1B2" {
		t.Fatalf("upper hash want A1B2 got %s", d.HashTag)
	}
	_ = strings.TrimSpace
}

func TestValidateSeen(t *testing.T) {
	seen := [][2]int{{1, 20}}
	if err := ValidateSeen(Directive{Start: 10, End: 15}, seen); err != nil {
		t.Fatalf("accepted failed: %v", err)
	}
	if err := ValidateSeen(Directive{Start: 1, End: 20}, seen); err != nil {
		t.Fatalf("edge failed: %v", err)
	}
	if err := ValidateSeen(Directive{Start: 50, End: 60}, seen); err == nil {
		t.Fatal("unseen should fail")
	}
	if err := ValidateSeen(Directive{Start: 15, End: 25}, [][2]int{{1, 20}}); err == nil {
		t.Fatal("partial overlap should fail")
	}
	if err := ValidateSeen(Directive{Start: 1, End: 1}, nil); err == nil {
		t.Fatal("no seen should fail")
	}
	// multiple ranges
	seen2 := [][2]int{{1, 10}, {20, 30}}
	if err := ValidateSeen(Directive{Start: 25, End: 28}, seen2); err != nil {
		t.Fatalf("second range: %v", err)
	}
	if err := ValidateSeen(Directive{Start: 15, End: 15}, seen2); err == nil {
		t.Fatal("gap should fail")
	}
}
