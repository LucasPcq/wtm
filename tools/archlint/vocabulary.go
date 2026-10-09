package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// glyphVocabulary is docs/dev/output.md's table, as runes. A seventh glyph is a
// decision; typing one is not.
var glyphVocabulary = map[string]string{
	"✓": "GlyphSuccess",
	"!": "GlyphAttention",
	"✗": "GlyphFailure",
	"=": "GlyphUnchanged",
	"›": "GlyphProgress",
	"→": "NextStepGlyph (a line head) or MoveArrowGlyph (punctuation inside a value)",
	"~": "GlyphUpdate",
}

// drawingLayers are the ones that put glyphs on a screen. rules/ and service/
// are left out on purpose: `=` and `!` are ordinary bytes to an env parser or a
// pnpm workspace pattern, and a rule that cannot tell those apart is a rule
// people work around.
var drawingLayers = map[string]bool{"styles": true, "surface/cli/render": true, "surface/tui": true}

var (
	outputMessage = objectRef{Path: internalPrefix + "surface/cli/render", Name: "Message"}
	mutedStyle    = objectRef{Path: internalPrefix + "styles", Name: "Muted"}
)

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

var tuistyleAnalyzer = &analysis.Analyzer{
	Name: "tuistyle",
	Doc:  "internal/surface/cli/render uses no badge or dashboard style",
	Run:  runTUIStyle,
}

func runTUIStyle(pass *analysis.Pass) (any, error) {
	if layerOfPackage(pass.Pkg.Path()) != "surface/cli/render" {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			obj := pass.TypesInfo.Uses[sel.Sel]
			if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != internalPrefix+"styles" {
				return true
			}
			name := sel.Sel.Name
			if strings.HasPrefix(name, "Badge") || strings.HasPrefix(name, "Dashboard") {
				pass.Reportf(sel.Pos(), "internal/surface/cli/render must not use styles.%s: a badge is a TUI widget and a dashboard style belongs to that surface — a line of CLI output is text", name)
			}
			return true
		})
	}
	return nil, nil
}

var mutedlineAnalyzer = &analysis.Analyzer{
	Name: "mutedline",
	Doc:  "a bare line is never muted whole",
	Run:  runMutedLine,
}

func runMutedLine(pass *analysis.Pass) (any, error) {
	if !drawingLayers[layerOfPackage(pass.Pkg.Path())] {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 || !refers(objectOf(pass.TypesInfo, call.Fun), outputMessage) {
				return true
			}
			if rendersMutedWhole(pass.TypesInfo, call.Args[1]) {
				pass.Reportf(call.Pos(), "a bare line muted whole is the `=` register without its glyph: use render.Unchanged for a non-event, or subordinate detail with an indent — see docs/dev/output.md")
			}
			return true
		})
	}
	return nil, nil
}

// rendersMutedWhole is `styles.Muted.Render(x)` as the whole argument. A muted
// fragment concatenated with content is a label, which is what Muted is for.
func rendersMutedWhole(info *types.Info, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	render, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || render.Sel.Name != "Render" {
		return false
	}
	return refers(objectOf(info, render.X), mutedStyle)
}
