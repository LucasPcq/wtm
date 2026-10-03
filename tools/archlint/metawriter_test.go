package main

import "testing"

func TestEveryMetadataWriterIsAMutator(t *testing.T) {
	runTxtar(t, metawriterAnalyzer, "metawriter",
		internalPrefix+"service/worktree",
		internalPrefix+"service/env",
	)
}
