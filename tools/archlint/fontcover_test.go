package main

import (
	"testing"
	"unicode/utf8"
)

func TestARuneMissingFromCommonFontsIsReported(t *testing.T) {
	results := runTxtar(t, fontcoverAnalyzer, "fontcover", internalPrefix+"rules/fc")
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
