package main

import "testing"

func TestAGlyphIsWrittenThroughItsConstantInADrawingLayer(t *testing.T) {
	runTxtar(t, glyphAnalyzer, "vocabulary", internalPrefix+"surface/tui/glyphs", internalPrefix+"rules/plain")
}

func TestOutputUsesNoTUIStyleEvenThroughAnAlias(t *testing.T) {
	runTxtar(t, tuistyleAnalyzer, "vocabulary", internalPrefix+"surface/cli/render/badge")
}

func TestABareLineIsNeverMutedWhole(t *testing.T) {
	runTxtar(t, mutedlineAnalyzer, "vocabulary", internalPrefix+"surface/cli/render")
}
