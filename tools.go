//go:build tools

package tools

import (
	_ "github.com/biggs-100/biggz-ai/tools/nosourcegrep"
	_ "github.com/biggs-100/biggz-ai/tools/nosourcegrep/cmd/nosourcegrep"
	_ "github.com/fzipp/gocyclo/cmd/gocyclo"
	_ "github.com/uudashr/gocognit/cmd/gocognit"
	_ "golang.org/x/tools/go/analysis"
)
