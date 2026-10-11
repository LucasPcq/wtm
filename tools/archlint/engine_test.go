package main

import "testing"

func TestTheUseCasesAreReachedThroughDispatchOnly(t *testing.T) {
	runTxtar(t, engineAnalyzer, "engine",
		internalPrefix+"command/create",
		internalPrefix+"query/operations",
		internalPrefix+"surface/cli",
		internalPrefix+"dispatch",
	)
}
