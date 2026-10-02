package main

import "testing"

func TestAGlyphIsWrittenThroughItsConstantInADrawingLayer(t *testing.T) {
	runTxtar(t, glyphAnalyzer, "vocabulary", internalPrefix+"tui/glyphs", internalPrefix+"rules/plain")
}

func TestOutputUsesNoTUIStyleEvenThroughAnAlias(t *testing.T) {
	runTxtar(t, tuistyleAnalyzer, "vocabulary", internalPrefix+"output/badge")
}

func TestABareLineIsNeverMutedWhole(t *testing.T) {
	runTxtar(t, mutedlineAnalyzer, "vocabulary", internalPrefix+"output")
}
