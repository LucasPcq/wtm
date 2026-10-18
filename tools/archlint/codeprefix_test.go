package main

import "testing"

func TestACodeOfACommandStartsWithItsName(t *testing.T) {
	runTxtar(t, codeprefixAnalyzer, "codeprefix",
		nextPrefix+"internal/command/create",
		nextPrefix+"internal/command/create/cli",
		nextPrefix+"internal/command/fastforward",
		nextPrefix+"internal/command/fastforward/cli",
		nextPrefix+"internal/service/provision",
	)
}
