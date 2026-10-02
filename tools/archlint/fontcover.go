package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

// fontSafe is every non-letter rune a string may put on a screen. It was
// measured, not chosen: present in all of Cascadia Code, DejaVu Sans Mono, Fira
// Code, Hack, IBM Plex Mono, Inconsolata, JetBrains Mono, Roboto Mono, Source
// Code Pro, Ubuntu Mono, Menlo, Monaco and SF Mono. A rune a font lacks is drawn
// from a fallback face, often wider than the cell, and eats the space after it —
// which is how `↻ Updated profile` lost its space in Ghostty.
var fontSafe = map[rune]string{
	'§': "", '·': "", '×': "", '—': "", '•': "", '…': "", '‹': "", '›': "", '−': "",
	'✓': "missing from Hack, Monaco and Roboto/Ubuntu Mono, kept as the success glyph every CLI uses",
	'✗': "missing from 8 of the 13, kept as the failure glyph every CLI uses",
	'←': "missing only from Monaco and Roboto/Ubuntu Mono",
	'↑': "missing only from Monaco and Roboto/Ubuntu Mono",
	'→': "missing only from Roboto/Ubuntu Mono",
	'↓': "missing only from Roboto/Ubuntu Mono",
}

// fontLegacy predates the rule, each rune with the number of sites it had. Each
// reports once as migrating; a site beyond its count fails, and a count higher
// than needed is reported to be lowered — the list may only shrink, towards a
// rune of fontSafe.
var fontLegacy = map[rune]int{
	'↗': 1, '⊘': 1, '⋯': 2, '▸': 21, '▾': 2, '▶': 1, '◆': 1,
	'◈': 1, '◉': 1, '○': 5, '◌': 1, '●': 12, '⚠': 20, '❯': 2,
}

func legacyBudgets() map[string]int {
	budgets := make(map[string]int, len(fontLegacy))
	for r, sites := range fontLegacy {
		budgets[string(r)] = sites
	}
	return budgets
}

// isTerminalDrawn is box drawing and block elements, which terminals render as
// their own sprites rather than from the font.
func isTerminalDrawn(r rune) bool {
	return r >= 0x2500 && r <= 0x259F
}

var fontcoverAnalyzer = &analysis.Analyzer{
	Name: "fontcover",
	Doc:  "a string puts on screen only runes common monospace fonts carry",
	Run:  runFontCover,
}

func runFontCover(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			lit, ok := node.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			text, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			for _, r := range uncoveredRunes(text) {
				d := analysis.Diagnostic{
					Pos:     lit.Pos(),
					Message: fmt.Sprintf("%q (U+%04X) is missing from common monospace fonts: the terminal draws it from a wider fallback face — use a rune of fontSafe, see docs/dev/output.md", r, r),
				}
				if _, legacy := fontLegacy[r]; legacy {
					d.Category = string(r)
				}
				pass.Report(d)
			}
			return true
		})
	}
	return nil, nil
}

func uncoveredRunes(text string) []rune {
	seen := map[rune]bool{}
	var runes []rune
	for _, r := range text {
		if r < 0x80 || seen[r] || unicode.IsLetter(r) || unicode.Is(unicode.Mn, r) || isTerminalDrawn(r) {
			continue
		}
		seen[r] = true
		if _, ok := fontSafe[r]; ok {
			continue
		}
		runes = append(runes, r)
	}
	return runes
}
