package main

import (
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestAPackageThatDoesNotTypeCheckIsAnError(t *testing.T) {
	if _, err := analyze(analyzeParams{Root: "testdata/broken", Systems: []string{"linux"}}); err == nil {
		t.Error("a package with a type error was analysed: archlint would print `clean` over code it never understood")
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
