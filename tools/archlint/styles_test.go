package main

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestOnlyStylesInstantiatesAStyleEvenThroughAnAlias(t *testing.T) {
	analysistest.Run(t, filepath.Join(analysistest.TestData(), "styles"), stylesAnalyzer,
		internalPrefix+"tui/bad",
		internalPrefix+"styles",
	)
}
