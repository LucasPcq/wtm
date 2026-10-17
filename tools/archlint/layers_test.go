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

func TestTheStoreWritesForTheServicesNeverForAFlow(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers",
		internalPrefix+"store",
		internalPrefix+"flow/leaky",
	)
}

func TestTheRulesDoNoIO(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers", internalPrefix+"rules/pure")
}

func TestAPackageWithoutARowIsAFinding(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers", internalPrefix+"newlayer")
}
