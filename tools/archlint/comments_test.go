package main

import "testing"

func TestTheEngineKeepsTheShapesOfItsComments(t *testing.T) {
	runTxtar(t, newCommentsAnalyzer(loadedTests), "comments",
		internalPrefix+"infra/fs",
		internalPrefix+"infra/proc",
		internalPrefix+"service/hooks",
		internalPrefix+"infra/git",
		internalPrefix+"registry",
		internalPrefix+"rules/runenv",
		internalPrefix+"service/github",
		internalPrefix+"rules",
	)
}
