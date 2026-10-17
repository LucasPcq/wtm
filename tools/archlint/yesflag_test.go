package main

import "testing"

func TestACommandReadingTheGateRegistersYes(t *testing.T) {
	runTxtar(t, yesflagAnalyzer, "yesflag",
		internalPrefix+"surface/cli/missing",
		internalPrefix+"surface/cli/registered",
		internalPrefix+"surface/cli/aliased",
		internalPrefix+"surface/cli/retired",
		internalPrefix+"surface/cli/helper",
		internalPrefix+"surface/cli/field",
	)
}
