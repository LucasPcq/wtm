package main

import "testing"

func TestEveryLowerPackageSaysItsResponsibility(t *testing.T) {
	runTxtar(t, pkgdocAnalyzer, "pkgdoc",
		internalPrefix+"service/hooks",
		internalPrefix+"service/github",
		internalPrefix+"infra/fs",
		internalPrefix+"store",
		internalPrefix+"rules",
	)
}
