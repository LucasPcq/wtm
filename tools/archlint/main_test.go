package main

import (
	"go/token"
	"testing"
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
