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
	"kernel": {
		noIO: true,
		why:  "the skeleton of the command engine: stdlib only, no business vocabulary, no I/O",
	},
	"command": {
		internal: append([]string{"config", "domain", "infra/fs", "infra/git", "infra/proc", "kernel", "rules", "store"}, engineServicePaths()...),
		why:      "a use case of the engine reaches the world through its ports — store/, infra/git, infra/fs, infra/proc and the engine's services — never through flow/, a surface, the git functions at the root of infra/ or the services of the commands on flow/",
	},
	"query": {
		internal: append([]string{"config", "domain", "infra/fs", "infra/git", "infra/proc", "kernel", "rules", "store"}, engineServicePaths()...),
		why:      "a read of the engine reaches the world through the ports a command does",
	},
	"registry": {
		internal: append([]string{"command", "config", "dispatch", "domain", "infra", "kernel", "query", "rules", "service/process", "store"}, engineServicePaths()...),
		why:      "the composition root: every command and query with its real ports, handed to dispatch; the only importer of command/ and query/, imported by the production wiring of the surfaces only; service/process answers the daemon's public proxy port until its client is the port infra/daemon (step 6 of LUC-283)",
	},
	"flow/legacy": {
		internal: []string{"dispatch", "domain", "flow", "kernel", "rules"},
		why:      "the one-way bridge of the commands still on flow/ to the engine: it reaches a command through dispatch, never command/, and nothing on the engine imports it",
	},
	"dispatch": {
		internal: []string{"domain", "infra", "kernel", "rules"},
		why:      "the impure half of the engine runs on its own ports: a command's ports reach it as interfaces, never through service/ or a surface",
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
		internal: []string{"domain", "flow", "kernel", "rules", "styles"},
		external: []string{"golang.org/x/term"},
		why:      "formats and prints, zero decision logic: it reads a command's results from the kernel, never runs one through dispatch",
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
	"surface/tui/form": {
		internal: []string{"dispatch", "domain", "kernel", "styles", "surface/tui"},
		external: []string{"github.com/charmbracelet/"},
		why:      "the form of a command on the engine, drawn with the wizard's components: it reaches the command through dispatch only, never flow/ or service/",
	},
	"surface/cli": {
		internal: []string{"config", "dispatch", "domain", "flow", "flow/legacy", "infra", "kernel", "registry", "rules", "schemas", "service", "styles", "surface/cli/render", "surface/tui", "surface/tui/form"},
		external: []string{"github.com/charmbracelet/bubbletea", "github.com/spf13/cobra", "golang.org/x/term"},
		why:      "the command line: the commands generated from the dispatch registry, and the flag wiring of the commands still on flow/",
	},
	"schemas": {
		why: "the embedded JSON Schema files",
	},
	"testutil": {
		internal: []string{"command", "dispatch", "domain", "flow", "infra", "kernel", "rules", "schemas", "store"},
		external: []string{"github.com/santhosh-tekuri/jsonschema/"},
		why:      "test doubles for the flow seams, the dispatch bench, the commands and the ports of infra/ and store/, and the validator a contract test checks a bundled schema with",
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

var commandEdges = map[string][]string{
	"create": {"provision"},
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

func engineServicePaths() []string {
	paths := make([]string, 0, len(engineServices))
	for _, service := range engineServices {
		paths = append(paths, string(service))
	}
	return paths
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
