package main

import "testing"

func TestStatewriteKeepsFileWritesInsideInfra(t *testing.T) {
	runTxtar(t, statewriteAnalyzer, "statewrite",
		internalPrefix+"infra/fs",
		internalPrefix+"service/bad",
		internalPrefix+"service/shell",
		internalPrefix+"service/selfupdate",
	)
}
