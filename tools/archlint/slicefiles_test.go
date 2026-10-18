package main

import "testing"

func TestTheRootOfACommandHoldsOnlyItsSliceFiles(t *testing.T) {
	runTxtar(t, slicefilesAnalyzer, "slicefiles",
		nextPrefix+"internal/command/create",
		nextPrefix+"internal/command/create/cli",
		nextPrefix+"internal/command/create/sub",
		nextPrefix+"internal/service/provision",
	)
}
