package main

import "testing"

func TestOnlyTheEdgesOfNextReadTheEnvironment(t *testing.T) {
	runTxtar(t, envreadAnalyzer, "envread",
		nextPrefix+"internal/service/provision",
		nextPrefix+"internal/domain/env",
		nextPrefix+"internal/command/create",
		nextPrefix+"internal/command/create/cli",
		nextPrefix+"internal/ui/cli",
		nextPrefix+"internal/ui/output",
		nextPrefix+"internal/ui/form",
		nextPrefix+"internal/adapter/git",
		nextPrefix+"internal/adapter/gittest",
		nextPrefix+"cmd/wtm-next",
		internalPrefix+"service/env",
	)
}
