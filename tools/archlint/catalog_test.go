package main

import "testing"

func TestEveryCodeOfTheEngineHasItsMessage(t *testing.T) {
	runTxtar(t, catalogAnalyzer, "catalog",
		internalPrefix+"kernel",
		internalPrefix+"kernel/text",
		internalPrefix+"command/create",
		internalPrefix+"query/operations",
		internalPrefix+"dispatch",
		internalPrefix+"surface/cli",
	)
}
