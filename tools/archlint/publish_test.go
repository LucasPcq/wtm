package main

import "testing"

func TestOnlyTheBusPublishes(t *testing.T) {
	runTxtar(t, publishAnalyzer, "publish",
		internalPrefix+"service/process",
		internalPrefix+"service/events",
		internalPrefix+"service/runjobs",
		internalPrefix+"flow",
		internalPrefix+"flow/create",
		internalPrefix+"surface/tui/dash",
		internalPrefix+"dispatch",
		internalPrefix+"surface/cli/run",
		internalPrefix+"command/create",
		internalPrefix+"service/identity",
	)
}
