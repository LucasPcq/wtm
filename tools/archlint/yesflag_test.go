package main

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestACommandReadingTheGateRegistersYes(t *testing.T) {
	analysistest.Run(t, filepath.Join(analysistest.TestData(), "yesflag"), yesflagAnalyzer,
		internalPrefix+"commands/missing",
		internalPrefix+"commands/registered",
		internalPrefix+"commands/aliased",
		internalPrefix+"commands/retired",
		internalPrefix+"commands/helper",
		internalPrefix+"commands/field",
	)
}
