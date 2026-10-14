package main

import "testing"

func TestTheRootsOfRulesAndDomainOnlyShrink(t *testing.T) {
	runTxtar(t, rootfreezeAnalyzer, "rootfreeze",
		internalPrefix+"rules",
		internalPrefix+"rules/worktree",
		internalPrefix+"domain",
		internalPrefix+"domain/statefile",
	)
}
