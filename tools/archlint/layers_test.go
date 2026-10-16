package main

import "testing"

func TestALayerImportsOnlyWhatItsRowAllows(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers",
		internalPrefix+"rules/bad",
		internalPrefix+"flow/ok",
	)
}

func TestANestedLayerKeepsItsOwnRow(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers",
		internalPrefix+"surface/cli/render",
		internalPrefix+"surface/cli/wt",
	)
}

func TestTheDomainDeclaresNoFunction(t *testing.T) {
	runTxtar(t, domainAnalyzer, "domain", internalPrefix+"domain")
}

func TestTheKernelImportsNothingButTheStdlib(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers", internalPrefix+"kernel/text")
}

func TestDispatchRunsOnItsOwnPorts(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers", internalPrefix+"dispatch")
}

func TestTheRegistryAssemblesTheEngine(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers", internalPrefix+"registry")
}

func TestTheStoreWritesForTheServicesNeverForAFlow(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers",
		internalPrefix+"store",
		internalPrefix+"flow/leaky",
	)
}

func TestACommandReachesTheWorldThroughItsPortsOnly(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers",
		internalPrefix+"command/create",
		internalPrefix+"query/status",
	)
}

func TestTheRegistryIsReachedFromTheCommandLineOnly(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers",
		internalPrefix+"registry",
		internalPrefix+"surface/cli/root",
		internalPrefix+"surface/tui/dash",
		internalPrefix+"testutil/commandtest",
	)
}

func TestTheKernelAndTheRulesDoNoIO(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers",
		internalPrefix+"rules/pure",
		internalPrefix+"kernel",
	)
}

func TestAPackageWithoutARowIsAFinding(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers", internalPrefix+"newlayer")
}
