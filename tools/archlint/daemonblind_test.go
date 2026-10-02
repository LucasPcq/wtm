package main

import "testing"

func TestTheDaemonIsBlindToGit(t *testing.T) {
	runTxtar(t, daemonblindAnalyzer, "daemonblind",
		internalPrefix+"service/process",
		internalPrefix+"service/proxy",
		internalPrefix+"service/runjobs",
	)
}
