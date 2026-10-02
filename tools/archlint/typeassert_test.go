package main

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestOnlyAnUnguardedAssertionIsReported(t *testing.T) {
	analysistest.Run(t, filepath.Join(analysistest.TestData(), "typeassert"), typeassertAnalyzer, internalPrefix+"rules/x")
}
