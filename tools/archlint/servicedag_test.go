package main

import "testing"

func TestAServiceImportsAnotherOnlyAlongADeclaredEdge(t *testing.T) {
	runTxtar(t, servicedagAnalyzer, "servicedag",
		internalPrefix+"service/detect",
		internalPrefix+"service/process",
		internalPrefix+"service/process/processtest",
		internalPrefix+"service/fresh",
		internalPrefix+"flow/caller",
	)
}
