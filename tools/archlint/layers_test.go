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
