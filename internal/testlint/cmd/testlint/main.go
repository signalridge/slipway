package main

import (
	"github.com/signalridge/slipway/internal/testlint"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(testlint.Analyzer)
}
