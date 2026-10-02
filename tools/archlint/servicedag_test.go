package main

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAServiceImportsAnotherOnlyAlongADeclaredEdge(t *testing.T) {
	analysistest.Run(t, filepath.Join(analysistest.TestData(), "servicedag"), servicedagAnalyzer,
		internalPrefix+"service/detect",
		internalPrefix+"service/process",
		internalPrefix+"service/process/processtest",
		internalPrefix+"service/fresh",
		internalPrefix+"flow/caller",
	)
}
