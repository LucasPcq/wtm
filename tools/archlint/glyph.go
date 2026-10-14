package main

import (
	"go/ast"
	"go/token"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

var glyphVocabulary = map[string]string{
	"✓": "GlyphSuccess",
	"!": "GlyphAttention",
	"✗": "GlyphFailure",
	"=": "GlyphUnchanged",
	"›": "GlyphProgress",
	"→": "NextStepGlyph (a line head) or MoveArrowGlyph (punctuation inside a value)",
	"~": "GlyphUpdate",
}

var drawingLayers = map[string]bool{"styles": true, "surface/cli/render": true, "surface/tui": true}

var glyphAnalyzer = &analysis.Analyzer{
	Name: "glyph",
	Doc:  "a vocabulary glyph is written through its domain constant",
	Run:  runGlyph,
}

func runGlyph(pass *analysis.Pass) (any, error) {
	if !drawingLayers[layerOfPackage(pass.Pkg.Path())] {
		return nil, nil
	}
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
			if name, ok := glyphVocabulary[text]; ok {
				pass.Reportf(lit.Pos(), "the glyph %q is domain.%s — the vocabulary is declared once, see docs/dev/output.md", text, name)
			}
			return true
		})
	}
	return nil, nil
}
