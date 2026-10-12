package main

import "testing"

func TestGitexecKeepsGitInsideInfraGit(t *testing.T) {
	runTxtar(t, gitexecAnalyzer, "gitexec",
		internalPrefix+"infra/git",
		internalPrefix+"infra",
		internalPrefix+"service/bad",
		internalPrefix+"testutil/gittest",
	)
}
