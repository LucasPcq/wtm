package main

import "testing"

func TestAFlowCallingAMutatorPublishesItsEventAndTestsIt(t *testing.T) {
	runTxtar(t, emitsAnalyzer, "emits",
		internalPrefix+"flow/good",
		internalPrefix+"flow/silent",
		internalPrefix+"flow/untested",
	)
}
