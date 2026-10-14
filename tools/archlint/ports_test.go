package main

import "testing"

func TestACommandTakesTheLowerLayersPorts(t *testing.T) {
	runTxtar(t, portsAnalyzer, "ports",
		internalPrefix+"command/create",
		internalPrefix+"query/status",
		internalPrefix+"service/gitwt",
	)
}
