// Command archlint checks the architecture rules of CLAUDE.md that no
// general-purpose linter knows about. It is the mechanical half of section 9:
// the rules a reviewer would otherwise have to hold in their head on every PR,
// and which drift the moment nobody does.
//
// Usage: go run ./tools/archlint [-warn rule,rule] [dir]
//
// Each rule reports `file:line: [rule] message`, and a non-empty report exits
// 1. A rule named in -warn still reports but does not fail the run, for the one
// case where the codebase is mid-migration.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const modulePath = "github.com/LucasPcq/wtm/"

// layers is CLAUDE.md's dependency table, written once. A layer may import the
// internal packages listed and the external ones listed, plus the stdlib and
// itself. Anything else is a finding — so a new dependency is a deliberate edit
// here rather than something that lands unnoticed.
var layers = map[string]layer{
	"domain": {
		why: "types, errors and constants only",
	},
	"rules": {
		internal: []string{"domain"},
		why:      "pure functions over the domain: stdlib and internal/domain only, no I/O",
	},
	"infra": {
		internal: []string{"domain", "rules"},
		why:      "I/O, git exec, filesystem wrappers",
	},
	"config": {
		internal: []string{"domain", "infra", "rules"},
		external: []string{"github.com/BurntSushi/toml"},
		why:      "load and validate the config files",
	},
	"service": {
		internal: []string{"config", "domain", "infra", "rules"},
		external: []string{"github.com/creack/pty", "go.yaml.in/yaml/v3"},
		why:      "impure orchestration: no cobra, no bubbletea, no lipgloss",
	},
	"flow": {
		internal: []string{"domain", "rules", "service"},
		why:      "the run of a command, surface-independent: never cobra, bubbletea, lipgloss, output/, tui/, config/ or commands/ — and therefore never infra/, which needs a service/ wrapper instead",
	},
	"output": {
		internal: []string{"domain", "flow", "rules", "styles"},
		external: []string{"golang.org/x/term"},
		why:      "formats and prints, zero decision logic",
	},
	"styles": {
		internal: []string{"domain"},
		external: []string{"github.com/charmbracelet/lipgloss", "github.com/charmbracelet/x/ansi", "github.com/muesli/termenv"},
		why:      "the only package that instantiates a lipgloss.Style",
	},
	"tui": {
		internal: []string{"domain", "flow", "rules", "service", "styles"},
		external: []string{"github.com/charmbracelet/", "github.com/lrstanley/bubblezone", "golang.org/x/term"},
		why:      "bubbletea models, rendering only",
	},
	"commands": {
		internal: []string{"config", "domain", "flow", "infra", "output", "rules", "schemas", "service", "styles", "tui"},
		external: []string{"github.com/charmbracelet/bubbletea", "github.com/spf13/cobra", "golang.org/x/term"},
		why:      "flag wiring, delegating to flow/ and service/",
	},
	"schemas": {
		why: "the embedded JSON Schema files",
	},
	"testutil": {
		internal: []string{"domain", "flow"},
		why:      "test doubles for the flow seams",
	},
}

type layer struct {
	internal []string
	external []string
	why      string
}

type finding struct {
	pos  token.Position
	rule string
	msg  string
}

func main() {
	warn := flag.String("warn", "", "comma-separated rules that report without failing")
	flag.Parse()

	root := "internal"
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}

	warned := map[string]bool{}
	for _, name := range strings.Split(*warn, ",") {
		if name = strings.TrimSpace(name); name != "" {
			warned[name] = true
		}
	}

	migrating, err := loadMigrating()
	if err != nil {
		fmt.Fprintln(os.Stderr, "archlint:", err)
		os.Exit(2)
	}

	findings, err := check(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "archlint:", err)
		os.Exit(2)
	}

	failed := false
	for _, f := range findings {
		tag := ""
		switch {
		case warned[f.rule]:
			tag = " (warning)"
		case migrating.covers(f):
			tag = " (migrating)"
		default:
			failed = true
		}
		fmt.Printf("%s:%d:%d: [%s]%s %s\n", f.pos.Filename, f.pos.Line, f.pos.Column, f.rule, tag, f.msg)
	}
	if failed {
		fmt.Fprintf(os.Stderr, "\narchlint: %d finding(s) — see CLAUDE.md section 9\n", len(findings))
		os.Exit(1)
	}
	fmt.Printf("archlint: %s clean\n", root)
}

// migrating is what predates a rule and is tracked out rather than fixed on the
// spot. It reports without failing, and the list may only ever shrink: a new
// entry is a decision, not a workaround.
type migratingList struct {
	rules map[string][]*regexp.Regexp
}

func (m migratingList) covers(f finding) bool {
	for _, pattern := range m.rules[f.rule] {
		if pattern.MatchString(filepath.ToSlash(f.pos.Filename)) {
			return true
		}
	}
	return false
}

func loadMigrating() (migratingList, error) {
	list := migratingList{rules: map[string][]*regexp.Regexp{}}
	data, err := os.ReadFile(".archlint-migrating")
	if os.IsNotExist(err) {
		return list, nil
	}
	if err != nil {
		return list, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rule, pattern, ok := strings.Cut(line, " ")
		if !ok {
			return list, fmt.Errorf(".archlint-migrating: %q is not `<rule> <path regex>`", line)
		}
		compiled, err := regexp.Compile(strings.TrimSpace(pattern))
		if err != nil {
			return list, fmt.Errorf(".archlint-migrating: %w", err)
		}
		list.rules[rule] = append(list.rules[rule], compiled)
	}
	return list, nil
}

func check(root string) ([]finding, error) {
	fset := token.NewFileSet()
	var findings []finding

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil {
			return parseErr
		}
		findings = append(findings, checkFile(fset, path, file)...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].rule != findings[j].rule {
			return findings[i].rule < findings[j].rule
		}
		return findings[i].pos.String() < findings[j].pos.String()
	})
	return findings, nil
}

func checkFile(fset *token.FileSet, path string, file *ast.File) []finding {
	var findings []finding
	at := func(pos token.Pos) token.Position { return fset.Position(pos) }
	report := func(pos token.Pos, rule, format string, args ...any) {
		findings = append(findings, finding{pos: at(pos), rule: rule, msg: fmt.Sprintf(format, args...)})
	}

	own := layerOf(path)
	spec, known := layers[own]

	for _, imp := range file.Imports {
		target, _ := strconv.Unquote(imp.Path.Value)
		if !known {
			continue
		}
		if allowed(own, spec, target) {
			continue
		}
		report(imp.Pos(), "layers", "internal/%s must not import %q — %s", own, target, spec.why)
	}

	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncDecl:
			if own == "domain" {
				report(n.Pos(), "domain", "internal/domain declares %q: this layer holds types, errors and constants only — a function over the domain belongs in internal/rules", n.Name.Name)
			}
		case *ast.CallExpr:
			if own != "styles" && isSelector(n.Fun, "lipgloss", "NewStyle") {
				report(n.Pos(), "styles", "only internal/styles may instantiate a lipgloss.Style")
			}
		case *ast.CompositeLit:
			if own != "styles" && isQualified(n.Type, "lipgloss", "Style") {
				report(n.Pos(), "styles", "only internal/styles may instantiate a lipgloss.Style")
			}
		}
		return true
	})
	for _, pos := range unguardedAssertions(file) {
		report(pos, "typeassert", "type assertion without the comma-ok form: a wrong type must be an error, not a panic")
	}

	findings = append(findings, checkOutputVocabulary(fset, own, file)...)

	if own == "commands" {
		findings = append(findings, checkYesFlag(fset, path, file)...)
		findings = append(findings, checkMutation(fset, path, file)...)
	}
	return findings
}

// glyphVocabulary is docs/dev/output.md's table, as runes. A seventh glyph is a
// decision; typing one is not.
var glyphVocabulary = map[string]string{
	"✓": "GlyphSuccess",
	"!": "GlyphAttention",
	"✗": "GlyphFailure",
	"=": "GlyphUnchanged",
	"›": "GlyphProgress",
	"→": "NextStepGlyph (a line head) or MoveArrowGlyph (punctuation inside a value)",
	"↻": "GlyphUpdate",
}

// drawingLayers are the ones that put glyphs on a screen. rules/ and service/
// are left out on purpose: `=` and `!` are ordinary bytes to an env parser or a
// pnpm workspace pattern, and a rule that cannot tell those apart is a rule
// people work around.
var drawingLayers = map[string]bool{"output": true, "styles": true, "tui": true}

// checkOutputVocabulary is docs/dev/output.md made mechanical. The three rules
// it enforces are the ones the surface actually drifted on once the glyph table
// alone proved not to be enough:
//
//   - glyph: the vocabulary lives in domain, so a seventh rune cannot be
//     introduced by typing one, and an existing one cannot quietly take a
//     second meaning.
//   - tuistyle: a badge or a dashboard style is a widget's vocabulary. In a
//     line of CLI output a filled chip carries its own padding, which is what
//     made an attention line two columns wider — and louder — than the failure
//     line under it.
//   - mutedline: a bare line muted whole is the `=` register with its glyph
//     filed off. Muted has two jobs — chrome, and a non-event line that carries
//     its glyph — and secondary detail is subordinated by indentation.
func checkOutputVocabulary(fset *token.FileSet, own string, file *ast.File) []finding {
	if !drawingLayers[own] {
		return nil
	}

	var findings []finding
	report := func(pos token.Pos, rule, format string, args ...any) {
		findings = append(findings, finding{pos: fset.Position(pos), rule: rule, msg: fmt.Sprintf(format, args...)})
	}

	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.BasicLit:
			if own == "domain" || n.Kind != token.STRING {
				return true
			}
			text, err := strconv.Unquote(n.Value)
			if err != nil {
				return true
			}
			if name, ok := glyphVocabulary[text]; ok {
				report(n.Pos(), "glyph", "the glyph %q is domain.%s — the vocabulary is declared once, see docs/dev/output.md", text, name)
			}
		case *ast.SelectorExpr:
			if own != "output" {
				return true
			}
			pkg, ok := n.X.(*ast.Ident)
			if !ok || pkg.Name != "styles" {
				return true
			}
			if strings.HasPrefix(n.Sel.Name, "Badge") || strings.HasPrefix(n.Sel.Name, "Dashboard") {
				report(n.Pos(), "tuistyle", "internal/output must not use styles.%s: a badge is a TUI widget and a dashboard style belongs to that surface — a line of CLI output is text", n.Sel.Name)
			}
		case *ast.CallExpr:
			if !isMessageCall(n.Fun) || len(n.Args) != 2 {
				return true
			}
			if isMutedRender(n.Args[1]) {
				report(n.Pos(), "mutedline", "a bare line muted whole is the `=` register without its glyph: use output.Unchanged for a non-event, or subordinate detail with an indent — see docs/dev/output.md")
			}
		}
		return true
	})
	return findings
}

func isMessageCall(fun ast.Expr) bool {
	if ident, ok := fun.(*ast.Ident); ok {
		return ident.Name == "Message"
	}
	return isSelector(fun, "output", "Message")
}

// isMutedRender is `styles.Muted.Render(x)` — the whole argument and nothing
// else. A muted fragment concatenated with content is a label, which is exactly
// what Muted is for.
func isMutedRender(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	render, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || render.Sel.Name != "Render" {
		return false
	}
	return isSelector(render.X, "styles", "Muted") || isSelector(render.X, "", "Muted")
}

// unguardedAssertions is every `x.(T)` a wrong type would panic on. The two
// checked forms are subtracted rather than special-cased on the way down: a
// type switch owns its assertion, and a comma-ok one is only recognisable from
// the assignment above it.
func unguardedAssertions(file *ast.File) []token.Pos {
	guarded := map[token.Pos]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		var values []ast.Expr
		var targets int
		switch n := node.(type) {
		case *ast.AssignStmt:
			values, targets = n.Rhs, len(n.Lhs)
		case *ast.ValueSpec:
			values, targets = n.Values, len(n.Names)
		case *ast.TypeSwitchStmt:
			ast.Inspect(n.Assign, func(inner ast.Node) bool {
				if assert, ok := inner.(*ast.TypeAssertExpr); ok {
					guarded[assert.Pos()] = true
				}
				return true
			})
			return true
		default:
			return true
		}
		if targets != 2 || len(values) != 1 {
			return true
		}
		if assert, ok := values[0].(*ast.TypeAssertExpr); ok {
			guarded[assert.Pos()] = true
		}
		return true
	})

	var unguarded []token.Pos
	ast.Inspect(file, func(node ast.Node) bool {
		assert, ok := node.(*ast.TypeAssertExpr)
		// A nil Type is the `x.(type)` of a type switch, which has no wrong-type
		// case to guard.
		if ok && assert.Type != nil && !guarded[assert.Pos()] {
			unguarded = append(unguarded, assert.Pos())
		}
		return true
	})
	return unguarded
}

// checkYesFlag enforces the confirmation axis: a file that declares a command
// and reads the prompt-capability gate must register --yes on it. Reading the
// gate without offering the flag is a command that decides whether to ask
// without letting the caller answer "do not".
func checkYesFlag(fset *token.FileSet, path string, file *ast.File) []finding {
	if !declaresCommand(file) {
		return nil
	}
	src := renderCalls(file)
	if !src["shared.Interactive"] && !src["interactiveRun"] {
		return nil
	}
	if src["shared.AddYesFlag"] || src["shared.AddNoPromptFlags"] || src["AddYesFlag"] {
		return nil
	}
	return []finding{{
		pos:  fset.Position(file.Pos()),
		rule: "yesflag",
		msg:  fmt.Sprintf("%s reads the interactive gate but registers no --yes: the confirmation axis is a flag, not an inference", filepath.Base(path)),
	}}
}

// declaresCommand reports whether the file builds a command, as opposed to
// merely taking one as a parameter: a helper shared by several commands has no
// flags of its own to register.
func declaresCommand(file *ast.File) bool {
	built := false
	ast.Inspect(file, func(node ast.Node) bool {
		if lit, ok := node.(*ast.CompositeLit); ok && isQualified(lit.Type, "cobra", "Command") {
			built = true
		}
		return !built
	})
	return built
}

// mutations are the service calls that change a worktree. A command reaching
// for one directly has put its flow in the runner: what it does can then only
// be replayed by a cobra command, which is what internal/flow exists to undo.
var mutations = map[string][]string{
	"worktree": {"Create", "Clean", "ForceClean", "Sync", "Relocate", "Extract", "Reparent", "Remove", "Move"},
	"envsvc":   {"ApplyEnvSync", "ApplyEnvPorts"},
}

// checkMutation enforces "every worktree-mutating command goes through flow/".
// The commands that predate the rule are listed in .archlint-migrating, each
// with the ticket that moves it — the list may only shrink.
func checkMutation(fset *token.FileSet, path string, file *ast.File) []finding {
	var findings []finding
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		for pkg, names := range mutations {
			for _, name := range names {
				if !isSelector(call.Fun, pkg, name) {
					continue
				}
				findings = append(findings, finding{
					pos:  fset.Position(call.Pos()),
					rule: "mutation",
					msg: fmt.Sprintf("%s.%s is called from commands/: a worktree-mutating command goes through internal/flow/<cmd>, "+
						"or no second surface can ever run it — see CLAUDE.md, \"Every new worktree-mutating command goes through flow/\"", pkg, name),
				})
			}
		}
		return true
	})
	return findings
}

// renderCalls indexes the selectors and composite types a file mentions, which
// is all the yesflag rule needs to know about it.
func renderCalls(file *ast.File) map[string]bool {
	seen := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.SelectorExpr:
			if ident, ok := n.X.(*ast.Ident); ok {
				seen[ident.Name+"."+n.Sel.Name] = true
			}
		case *ast.Ident:
			seen[n.Name] = true
		}
		return true
	})
	return seen
}

func allowed(own string, spec layer, target string) bool {
	if !strings.Contains(strings.Split(target, "/")[0], ".") {
		return true // stdlib
	}
	if strings.HasPrefix(target, modulePath+"internal/") {
		other := strings.Split(strings.TrimPrefix(target, modulePath+"internal/"), "/")[0]
		if other == own {
			return true
		}
		return contains(spec.internal, other)
	}
	for _, prefix := range spec.external {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func layerOf(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		if part == "internal" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == pkg
}

func isQualified(expr ast.Expr, pkg, name string) bool {
	return isSelector(expr, pkg, name)
}
