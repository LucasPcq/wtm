package main

import "testing"

func TestOutputUsesNoTUIStyleEvenThroughAnAlias(t *testing.T) {
	runTxtar(t, tuistyleAnalyzer, "vocabulary", internalPrefix+"surface/cli/render/badge")
}
