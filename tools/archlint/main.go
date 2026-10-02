// Command archlint checks the architecture rules of CLAUDE.md that no
// general-purpose linter knows about: the mechanical half of section 9.
//
// Usage: go run ./tools/archlint [-warn rule,rule] [dir]
//
// Each rule is a go/analysis Analyzer and reports `file:line:col: [rule] message`;
// a non-empty report exits 1. A rule named in -warn reports without failing.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
)

// targetSystems are loaded in turn because go/packages keeps only the files a
// build would compile: peerpid_linux.go and the proxy's *_other.go would drop
// out on a Mac, which the parse-everything walk this replaced never let happen.
var targetSystems = []string{"darwin", "linux"}

const loadMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
	packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes |
	packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule

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

	findings, err := analyze(analyzeParams{Root: root, Systems: targetSystems})
	if err != nil {
		fmt.Fprintln(os.Stderr, "archlint:", err)
		os.Exit(2)
	}

	verdict := judge(judgeParams{Findings: findings, Migrating: migrating, Warned: warned})
	for _, line := range verdict.lines {
		fmt.Println(line)
	}
	for _, note := range verdict.notes {
		fmt.Println("archlint: note:", note)
	}
	if verdict.failed {
		fmt.Fprintf(os.Stderr, "\narchlint: %d finding(s) — see CLAUDE.md section 9\n", len(findings))
		os.Exit(1)
	}
	fmt.Printf("archlint: %s clean\n", root)
}

type analyzeParams struct {
	Root    string
	Systems []string
}

func analyze(params analyzeParams) ([]finding, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	var batches [][]finding
	for _, goos := range params.Systems {
		batch, err := analyzeFor(analyzeForParams{Root: params.Root, GOOS: goos, Cwd: cwd})
		if err != nil {
			return nil, err
		}
		batches = append(batches, batch)
	}
	findings := collapseLegacy(collapseLegacyParams{Findings: mergeFindings(batches), Budgets: legacyBudgets()})
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].rule != findings[j].rule {
			return findings[i].rule < findings[j].rule
		}
		return findings[i].pos.String() < findings[j].pos.String()
	})
	return findings, nil
}

type analyzeForParams struct {
	Root string
	GOOS string
	Cwd  string
}

func analyzeFor(params analyzeForParams) ([]finding, error) {
	config := &packages.Config{Mode: loadMode, Env: append(os.Environ(), "GOOS="+params.GOOS, "CGO_ENABLED=0")}
	pkgs, err := packages.Load(config, "./"+filepath.ToSlash(params.Root)+"/...")
	if err != nil {
		return nil, err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, errors.New("the packages above do not load: archlint analyses only code that type-checks")
	}
	graph, err := checker.Analyze(analyzers, pkgs, nil)
	if err != nil {
		return nil, err
	}
	var findings []finding
	for action := range graph.All() {
		if !action.IsRoot {
			continue
		}
		if action.Err != nil {
			return nil, fmt.Errorf("%s: %w", action, action.Err)
		}
		for _, d := range action.Diagnostics {
			pos := action.Package.Fset.Position(d.Pos)
			if rel, err := filepath.Rel(params.Cwd, pos.Filename); err == nil {
				pos.Filename = rel
			}
			findings = append(findings, finding{pos: pos, rule: action.Analyzer.Name, msg: d.Message, legacy: d.Category})
		}
	}
	return findings, nil
}

func mergeFindings(batches [][]finding) []finding {
	seen := map[string]bool{}
	var merged []finding
	for _, batch := range batches {
		for _, f := range batch {
			key := f.rule + "\x00" + f.pos.String() + "\x00" + f.msg
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, f)
		}
	}
	return merged
}
