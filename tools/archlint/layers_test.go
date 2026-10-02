package main

import "testing"

func TestALayerImportsOnlyWhatItsRowAllows(t *testing.T) {
	runTxtar(t, layersAnalyzer, "layers",
		internalPrefix+"rules/bad",
		internalPrefix+"flow/ok",
	)
}

func TestTheDomainDeclaresNoFunction(t *testing.T) {
	runTxtar(t, domainAnalyzer, "domain", internalPrefix+"domain")
}
