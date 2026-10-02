package main

import "testing"

func TestOnlyAnUnguardedAssertionIsReported(t *testing.T) {
	runTxtar(t, typeassertAnalyzer, "typeassert", internalPrefix+"rules/x")
}
