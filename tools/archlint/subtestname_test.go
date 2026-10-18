package main

import "testing"

func TestASubtestOfNextIsNamedLikeAnIdentifier(t *testing.T) {
	runTxtar(t, newSubtestnameAnalyzer(loadedTests), "subtestname",
		nextPrefix+"internal/domain/env",
		nextPrefix+"internal/service/operationtest",
		nextPrefix+"internal/service/provision",
		internalPrefix+"rules/env",
	)
}
