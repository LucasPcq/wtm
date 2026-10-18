package main

import (
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestAPackageThatDoesNotTypeCheckIsAnError(t *testing.T) {
	dir := t.TempDir()
	extract(t, dir, "broken")
	t.Chdir(dir)
	if _, err := analyze(analyzeParams{Roots: []string{"broken"}, Systems: []string{"linux"}}); err == nil {
		t.Error("a package with a type error was analysed: archlint would print `clean` over code it never understood")
	}
}

// analysistest loads the tests of a package, so only the driver, which does not, shows a directory of tests alone.
func TestADirectoryOfNextHoldingOnlyTestsIsRefused(t *testing.T) {
	dir := t.TempDir()
	extract(t, dir, "testonly")
	t.Chdir(dir)
	findings, err := analyze(analyzeParams{Roots: []string{"next"}, Systems: []string{"darwin"}})
	if err != nil {
		t.Fatal(err)
	}
	findings = slices.DeleteFunc(findings, func(f finding) bool { return f.rule != nextAnalyzer.Name })
	var refused []string
	for _, f := range findings {
		if strings.Contains(f.msg, "holds only tests") {
			refused = append(refused, f.pos.Filename)
		}
	}
	// a file compiled on no target system leaves its directory with tests alone, as the driver sees it
	want := []string{"next/internal/command/ignored/x_test.go", "next/internal/command/windows/x_test.go", "next/internal/command/zzprobe/x_test.go"}
	if !slices.Equal(refused, want) || len(findings) != len(want) {
		t.Errorf("findings = %v, want %v refused for holding only tests", findings, want)
	}
}

func TestFindingsFromTwoSystemsAreMergedOnce(t *testing.T) {
	shared := finding{pos: token.Position{Filename: "internal/a.go", Line: 3, Column: 2}, rule: "typeassert", msg: "m"}
	linuxOnly := finding{pos: token.Position{Filename: "internal/a_linux.go", Line: 1, Column: 1}, rule: "typeassert", msg: "m"}
	got := mergeFindings([][]finding{{shared}, {shared, linuxOnly}})
	if len(got) != 2 {
		t.Errorf("merged = %v, want the shared finding once and the linux-only one kept", got)
	}
}

// A file no target system compiles is a file no rule ever reads: archlint would
// say `clean` about code it never saw.
func TestEveryFileOfTheTreeIsCompiledByATargetSystem(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	compiled := map[string]bool{}
	for _, goos := range targetSystems {
		config := &packages.Config{Mode: packages.NeedFiles | packages.NeedCompiledGoFiles, Dir: root, Env: append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=0")}
		pkgs, err := packages.Load(config, "./internal/...")
		if err != nil {
			t.Fatal(err)
		}
		for _, pkg := range pkgs {
			for _, file := range pkg.CompiledGoFiles {
				compiled[file] = true
			}
		}
	}
	err = filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "testdata" {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if !compiled[path] {
			t.Errorf("%s is compiled by none of %v", path, targetSystems)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
