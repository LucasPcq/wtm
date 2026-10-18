package main

import "testing"

func TestAPortFileOfNextDeclaresOnlyPorts(t *testing.T) {
	runTxtar(t, portfileAnalyzer, "portfile",
		nextPrefix+"internal/command/create",
		internalPrefix+"service/port",
	)
}
