package main

import "testing"

func TestTheEngineEmitsCodesNotText(t *testing.T) {
	runTxtar(t, enginetextAnalyzer, "enginetext",
		internalPrefix+"command/create",
		internalPrefix+"service/gitwt",
	)
}
