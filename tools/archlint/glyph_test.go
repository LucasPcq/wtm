package main

import "testing"

func TestAGlyphIsWrittenThroughItsConstantInADrawingLayer(t *testing.T) {
	runTxtar(t, glyphAnalyzer, "vocabulary", internalPrefix+"surface/tui/glyphs", internalPrefix+"rules/plain")
}
