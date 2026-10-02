package main

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestALayerImportsOnlyWhatItsRowAllows(t *testing.T) {
	analysistest.Run(t, filepath.Join(analysistest.TestData(), "layers"), layersAnalyzer,
		internalPrefix+"rules/bad",
		internalPrefix+"flow/ok",
	)
}

func TestTheDomainDeclaresNoFunction(t *testing.T) {
	analysistest.Run(t, filepath.Join(analysistest.TestData(), "domain"), domainAnalyzer, internalPrefix+"domain")
}
