package main

import "testing"

func TestABareLineIsNeverMutedWhole(t *testing.T) {
	runTxtar(t, mutedlineAnalyzer, "vocabulary", internalPrefix+"surface/cli/render")
}
