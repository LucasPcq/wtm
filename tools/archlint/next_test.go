package main

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestANextPackageImportsOnlyWhatItsRowAllows(t *testing.T) {
	runTxtar(t, newNextAnalyzer(loadedTests), "next",
		nextModule,
		nextPrefix+"internal/wtm/branch",
		nextPrefix+"internal/app/create",
		nextPrefix+"internal/app",
		nextPrefix+"internal/adapter/git",
		nextPrefix+"internal/adapter/gittest",
		nextPrefix+"internal/adapter/store",
		nextPrefix+"internal/surface/cli",
		nextPrefix+"cmd/wtm-next",
		nextPrefix+"internal/stray",
	)
}

func TestANextPackageKeepsItsCommentsUnderItsCeiling(t *testing.T) {
	ceilings := map[string]int{
		"internal/wtm/dotenv":     2,
		"internal/wtm/envcopy":    1,
		"internal/adapter/shell":  1,
		"internal/adapter/oplock": 2,
	}
	runTxtar(t, newNextCommentsAnalyzer(nextCommentsParams{Tests: loadedTests, Ceilings: func() (map[string]int, error) { return ceilings, nil }}), "nextcomments",
		nextPrefix+"internal/wtm/branch",
		nextPrefix+"internal/wtm/dotenv",
		nextPrefix+"internal/wtm/envcopy",
		nextPrefix+"internal/adapter/shell",
		nextPrefix+"internal/adapter/oplock",
	)
}

func TestTheCeilingsFileReadsOnePackageAndItsCountPerLine(t *testing.T) {
	ceilings, err := parseNextCeilings("# why each one earns its place: review\n\ninternal/adapter/shell 3\ninternal/wtm/dotenv   1\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]int{"internal/adapter/shell": 3, "internal/wtm/dotenv": 1}; !maps.Equal(ceilings, want) {
		t.Errorf("ceilings = %v, want %v", ceilings, want)
	}
}

func TestAMalformedCeilingIsAnError(t *testing.T) {
	for _, line := range []string{"internal/adapter/shell", "internal/adapter/shell x", "internal/adapter/shell -1", "internal/adapter/shell 1\ninternal/adapter/shell 2"} {
		if _, err := parseNextCeilings(line); err == nil {
			t.Errorf("%q parsed, want an error", line)
		}
	}
}

func TestTheRulesWrittenForInternalLeaveNextAlone(t *testing.T) {
	for _, a := range analyzers {
		if a.Name == typeassertAnalyzer.Name {
			continue
		}
		t.Run(a.Name, func(t *testing.T) {
			runTxtar(t, a, "nextlegacy",
				nextPrefix+"internal/adapter/git",
				nextPrefix+"internal/adapter/shell",
				nextPrefix+"internal/surface/output",
			)
		})
	}
}

func TestALanguageRuleStillActsOnNext(t *testing.T) {
	runTxtar(t, typeassertAnalyzer, "nextlegacy", nextPrefix+"internal/wtm/assert")
}

func TestACeilingWithoutItsPackageIsStale(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "wtm"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "wtm", "wtm.go"), []byte("package wtm\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stale := staleCeilings(staleCeilingsParams{Root: root, Ceilings: map[string]int{"internal/wtm": 1, "internal/gone": 2}})

	if want := []string{"internal/gone"}; !slices.Equal(stale, want) {
		t.Errorf("stale = %q, want %q", stale, want)
	}
}
