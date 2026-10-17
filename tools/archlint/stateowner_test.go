package main

import "testing"

func TestStateownerKeepsStateWritesInsideStore(t *testing.T) {
	runTxtar(t, stateownerAnalyzer, "stateowner",
		internalPrefix+"infra/fs",
		internalPrefix+"store",
		internalPrefix+"config",
		internalPrefix+"infra",
	)
}
