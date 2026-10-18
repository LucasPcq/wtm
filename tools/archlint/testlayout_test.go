package main

import "testing"

func TestATestFileOfNextHoldsOnlyTests(t *testing.T) {
	runTxtar(t, newTestlayoutAnalyzer(loadedTests), "testlayout",
		nextPrefix+"internal/domain/env",
		internalPrefix+"rules/env",
	)
}
