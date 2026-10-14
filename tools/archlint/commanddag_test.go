package main

import "testing"

func TestACommandImportsAnotherOnlyAlongADeclaredEdge(t *testing.T) {
	runTxtar(t, commanddagAnalyzer, "commanddag",
		internalPrefix+"command/create/createtest",
		internalPrefix+"command/provision",
	)
}
