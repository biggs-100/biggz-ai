package nosourcegrep_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/biggs-100/biggz-ai/tools/nosourcegrep"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, nosourcegrep.Analyzer, "bad", "good")
}
