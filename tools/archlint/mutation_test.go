package main

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestACommandCallingAMutatorIsReportedEvenThroughAnAlias(t *testing.T) {
	analysistest.Run(t, filepath.Join(analysistest.TestData(), "mutation"), mutationAnalyzer,
		internalPrefix+"commands/plain",
		internalPrefix+"commands/aliased",
		internalPrefix+"commands/method",
		internalPrefix+"tui/free",
	)
}
