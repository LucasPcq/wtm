package main

import "testing"

func TestEveryLowerPackageSaysItsResponsibility(t *testing.T) {
	runTxtar(t, pkgdocAnalyzer, "pkgdoc",
		internalPrefix+"service/gitwt",
		internalPrefix+"service/github",
		internalPrefix+"infra/git",
		internalPrefix+"store",
		internalPrefix+"rules",
		internalPrefix+"command/create",
		internalPrefix+"kernel",
	)
}
