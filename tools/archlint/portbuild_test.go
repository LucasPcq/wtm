package main

import "testing"

func TestARealPortIsBuiltByTheRegistryOnly(t *testing.T) {
	runTxtar(t, portbuildAnalyzer, "portbuild",
		internalPrefix+"command/create",
		internalPrefix+"registry",
		internalPrefix+"testutil/storetest",
		internalPrefix+"store",
	)
}
