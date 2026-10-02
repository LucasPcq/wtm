package main

import (
	"go/ast"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

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
		internal: []string{"domain", "infra", "rules", "schemas"},
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
		internal: []string{"domain", "flow", "schemas"},
		external: []string{"github.com/santhosh-tekuri/jsonschema/"},
		why:      "test doubles for the flow seams, and the validator a contract test checks a bundled schema with",
	},
}

// serviceEdges is the service row of the layers table split one level down:
// service/x may import service/y only along an edge declared here. A daemon
// that starts importing git, or a cycle in the making, is then an edit to this
// table rather than an import nobody saw.
var serviceEdges = map[string][]string{
	"detect":    {"branch"},
	"process":   {"proxy"},
	"runconfig": {"shellcmd"},
	"runjobs":   {"process", "runconfig", "worktree"},
	"worktree":  {"branch", "env", "github", "hooks", "process"},
}

type layer struct {
	internal []string
	external []string
	why      string
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

var layersAnalyzer = &analysis.Analyzer{
	Name: "layers",
	Doc:  "a layer imports only the packages its row of the layers table allows",
	Run:  runLayers,
}

func runLayers(pass *analysis.Pass) (any, error) {
	own := layerOfPackage(pass.Pkg.Path())
	spec, known := layers[own]
	if !known {
		return nil, nil
	}
	for _, file := range pass.Files {
		for _, imp := range file.Imports {
			target, err := strconv.Unquote(imp.Path.Value)
			if err != nil || allowed(own, spec, target) {
				continue
			}
			pass.Reportf(imp.Pos(), "internal/%s must not import %q — %s", own, target, spec.why)
		}
	}
	return nil, nil
}

var domainAnalyzer = &analysis.Analyzer{
	Name: "domain",
	Doc:  "internal/domain declares types, errors and constants only",
	Run:  runDomain,
}

func runDomain(pass *analysis.Pass) (any, error) {
	if layerOfPackage(pass.Pkg.Path()) != "domain" {
		return nil, nil
	}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			pass.Reportf(fn.Pos(), "internal/domain declares %q: this layer holds types, errors and constants only — a function over the domain belongs in internal/rules", fn.Name.Name)
		}
	}
	return nil, nil
}

var servicedagAnalyzer = &analysis.Analyzer{
	Name: "servicedag",
	Doc:  "a service package imports another only along a declared edge",
	Run:  runServiceDAG,
}

func runServiceDAG(pass *analysis.Pass) (any, error) {
	own := servicePackage(pass.Pkg.Path())
	if own == "" {
		return nil, nil
	}
	for _, file := range pass.Files {
		for _, imp := range file.Imports {
			target, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			other := servicePackage(target)
			if other == "" || other == own || slices.Contains(serviceEdges[own], other) {
				continue
			}
			pass.Reportf(imp.Pos(), "internal/service/%s must not import %q — undeclared service edge: declare it in serviceEdges (tools/archlint/layers.go), see CLAUDE.md section 9", own, target)
		}
	}
	return nil, nil
}

// servicePackage is the top-level service a package belongs to ("process" for
// service/process/processtest), or "" outside service/.
func servicePackage(pkgPath string) string {
	if !dir("service").holds(pkgPath) {
		return ""
	}
	parts := strings.Split(internalPath(pkgPath), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
