package main

import "testing"

func TestOnlyStylesInstantiatesAStyleEvenThroughAnAlias(t *testing.T) {
	runTxtar(t, stylesAnalyzer, "styles",
		internalPrefix+"tui/bad",
		internalPrefix+"styles",
	)
}
