package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

var fontSafe = map[rune]string{
	'§': "", '·': "", '×': "", '—': "", '•': "", '…': "", '‹': "", '›': "", '−': "",
	'✓': "missing from Hack, Monaco and Roboto/Ubuntu Mono, kept as the success glyph every CLI uses",
	'✗': "missing from 8 of the 13, kept as the failure glyph every CLI uses",
	'←': "missing only from Monaco and Roboto/Ubuntu Mono",
	'↑': "missing only from Monaco and Roboto/Ubuntu Mono",
	'→': "missing only from Roboto/Ubuntu Mono",
	'↓': "missing only from Roboto/Ubuntu Mono",
}

var fontLegacy = map[rune]int{
	'↗': 1, '⊘': 1, '⋯': 2, '▸': 21, '▾': 2, '▶': 1, '◆': 1,
	'◈': 1, '◉': 1, '○': 5, '◌': 1, '●': 12, '⚠': 17, '❯': 2,
}

func legacyBudgets() map[string]int {
	budgets := make(map[string]int, len(fontLegacy))
	for r, sites := range fontLegacy {
		budgets[string(r)] = sites
	}
	return budgets
}

// Box drawing and block elements are rendered by terminals as their own
// sprites rather than from the font.
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
