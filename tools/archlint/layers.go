package main

import (
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var layers = map[string]layer{
	"domain": {
		why: "types, errors and constants only",
	},
	"rules": {
		internal: []string{"domain"},
		noIO:     true,
		why:      "pure functions over the domain: stdlib and internal/domain only, no I/O",
	},
	"infra": {
		internal: []string{"domain", "rules"},
		external: []string{"golang.org/x/sys/unix"},
		why:      "I/O, git exec, filesystem wrappers",
	},
	"config": {
		internal: []string{"domain", "infra", "rules", "schemas"},
		external: []string{"github.com/BurntSushi/toml"},
		why:      "load and validate the config files",
	},
	"store": {
		internal: []string{"config", "domain", "infra", "rules"},
		why:      "the one writer of each state file: config/ keeps loading and validating, store/ writes",
	},
	"service": {
		internal: []string{"config", "domain", "infra", "rules", "store"},
		external: []string{"github.com/creack/pty", "go.yaml.in/yaml/v3"},
		why:      "impure orchestration: no cobra, no bubbletea, no lipgloss",
	},
	"flow": {
		internal: []string{"domain", "rules", "service"},
		why:      "the run of a command, surface-independent: never cobra, bubbletea, lipgloss, surface/ or config/ — and therefore never infra/, which needs a service/ wrapper instead",
	},
	"surface/cli/render": {
		internal: []string{"domain", "flow", "rules", "styles"},
		external: []string{"golang.org/x/term"},
		why:      "formats and prints, zero decision logic",
	},
	"styles": {
		internal: []string{"domain"},
		external: []string{"github.com/charmbracelet/lipgloss", "github.com/charmbracelet/x/ansi", "github.com/muesli/termenv"},
		why:      "the only package that instantiates a lipgloss.Style",
	},
	"surface/tui": {
		internal: []string{"domain", "flow", "rules", "service", "styles"},
		external: []string{"github.com/charmbracelet/", "github.com/lrstanley/bubblezone", "golang.org/x/term"},
		why:      "bubbletea models, rendering only",
	},
	"surface/cli": {
		internal: []string{"config", "domain", "flow", "infra", "rules", "schemas", "service", "styles", "surface/cli/render", "surface/tui"},
		external: []string{"github.com/charmbracelet/bubbletea", "github.com/spf13/cobra", "golang.org/x/term"},
		why:      "the command line: flag wiring to flow/, zero business logic",
	},
	"schemas": {
		why: "the embedded JSON Schema files",
	},
	"testutil": {
		internal: []string{"domain", "flow", "infra", "rules", "schemas", "store"},
		external: []string{"github.com/santhosh-tekuri/jsonschema/"},
		why:      "test doubles for the flow seams and infra/, and the validator a contract test checks a bundled schema with",
	},
}

var serviceEdges = map[string][]string{
	"detect":    {"branch"},
	"events":    {"process", "worktree"},
	"process":   {"proxy"},
	"runconfig": {"shellcmd"},
	"runjobs":   {"process", "runconfig", "worktree"},
	"worktree":  {"branch", "env", "github", "hooks", "process"},
}

type layer struct {
	internal []string
	external []string
	noIO     bool
	why      string
}

// pureNetwork parse an address and nothing more.
var (
	ioPackages  = []string{"os", "io/ioutil", "net", "syscall"}
	pureNetwork = []string{"net/url", "net/netip"}
)

func allowed(own string, spec layer, target string) bool {
	if !strings.Contains(strings.Split(target, "/")[0], ".") {
		return !spec.noIO || !doesIO(target)
	}
	if strings.HasPrefix(target, internalPrefix) {
		other := layerOfPackage(target)
		return other == own || admits(spec.internal, target)
	}
	for _, prefix := range spec.external {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
}

func admits(entries []string, target string) bool {
	other := layerOfPackage(target)
	return slices.ContainsFunc(entries, func(entry string) bool {
		return entry == other || (len(entry) > len(other) && dir(entry).holds(target))
	})
}

func doesIO(stdPkg string) bool {
	under := func(root string) bool { return stdPkg == root || strings.HasPrefix(stdPkg, root+"/") }
	return slices.ContainsFunc(ioPackages, under) && !slices.Contains(pureNetwork, stdPkg)
}

var layersAnalyzer = &analysis.Analyzer{
	Name: "layers",
	Doc:  "a layer imports only the packages its row of the layers table allows",
	Run:  runLayers,
}

func runLayers(pass *analysis.Pass) (any, error) {
	if internalPath(pass.Pkg.Path()) == "" || len(pass.Files) == 0 {
		return nil, nil
	}
	own := layerOfPackage(pass.Pkg.Path())
	spec, known := layers[own]
	if !known {
		pass.Reportf(firstFile(pass).Package, "internal/%s has no row in the layers table: declare what it may import in tools/archlint/layers.go (docs/dev/architecture.md)", internalPath(pass.Pkg.Path()))
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
