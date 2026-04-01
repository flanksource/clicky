package main

import (
	"github.com/flanksource/clicky/lint"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(lint.Analyzer)
}
