package main

import "testing"

func TestOnlyStylesInstantiatesAStyleEvenThroughAnAlias(t *testing.T) {
	runTxtar(t, stylesAnalyzer, "styles",
		internalPrefix+"surface/tui/bad",
		internalPrefix+"styles",
	)
}
