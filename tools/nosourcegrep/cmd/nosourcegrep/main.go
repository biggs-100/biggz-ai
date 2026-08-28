package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/biggs-100/biggz-ai/tools/nosourcegrep"
)

func main() { singlechecker.Main(nosourcegrep.Analyzer) }
