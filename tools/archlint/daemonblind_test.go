package main

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestTheDaemonIsBlindToGit(t *testing.T) {
	analysistest.Run(t, filepath.Join(analysistest.TestData(), "daemonblind"), daemonblindAnalyzer,
		internalPrefix+"service/process",
		internalPrefix+"service/runjobs",
	)
}
