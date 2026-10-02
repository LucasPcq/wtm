package main

import (
	"path/filepath"
	"testing"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestARuneMissingFromCommonFontsIsReported(t *testing.T) {
	results := analysistest.Run(t, filepath.Join(analysistest.TestData(), "fontcover"), fontcoverAnalyzer, internalPrefix+"rules/fc")
	categories := map[string]string{}
	for _, result := range results {
		for _, d := range result.Diagnostics {
			r, _ := utf8.DecodeRuneInString(d.Message[1:])
			categories[string(r)] = d.Category
		}
	}
	if got := categories["▸"]; got != "▸" {
		t.Errorf("legacy category of ▸ = %q, want it keyed so its sites collapse to one line", got)
	}
	if got := categories["↻"]; got != "" {
		t.Errorf("category of ↻ = %q, want none: it is not in fontLegacy and must fail", got)
	}
}
