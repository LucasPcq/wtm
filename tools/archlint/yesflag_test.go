package main

import "testing"

func TestACommandReadingTheGateRegistersYes(t *testing.T) {
	runTxtar(t, yesflagAnalyzer, "yesflag",
		internalPrefix+"commands/missing",
		internalPrefix+"commands/registered",
		internalPrefix+"commands/aliased",
		internalPrefix+"commands/retired",
		internalPrefix+"commands/helper",
		internalPrefix+"commands/field",
	)
}
