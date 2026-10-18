package main

import "testing"

func TestASubtestOfNextIsNamedLikeAnIdentifier(t *testing.T) {
	runTxtar(t, newSubtestnameAnalyzer(loadedTests), "subtestname",
		nextPrefix+"internal/domain/env",
		internalPrefix+"rules/env",
	)
}
