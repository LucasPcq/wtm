package main

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func vocabularyData() string { return filepath.Join(analysistest.TestData(), "vocabulary") }

func TestAGlyphIsWrittenThroughItsConstantInADrawingLayer(t *testing.T) {
	analysistest.Run(t, vocabularyData(), glyphAnalyzer, internalPrefix+"tui/glyphs", internalPrefix+"rules/plain")
}

func TestOutputUsesNoTUIStyleEvenThroughAnAlias(t *testing.T) {
	analysistest.Run(t, vocabularyData(), tuistyleAnalyzer, internalPrefix+"output/badge")
}

func TestABareLineIsNeverMutedWhole(t *testing.T) {
	analysistest.Run(t, vocabularyData(), mutedlineAnalyzer, internalPrefix+"output")
}
