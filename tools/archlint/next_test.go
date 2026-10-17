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
		nextPrefix+"internal/domain/branch",
		nextPrefix+"internal/service/provision",
		nextPrefix+"internal/command/create",
		nextPrefix+"internal/adapter/git",
		nextPrefix+"internal/adapter/gittest",
		nextPrefix+"internal/adapter/store",
		nextPrefix+"internal/ui/output",
		nextPrefix+"internal/ui/cli",
		nextPrefix+"internal/command/create/cli",
		nextPrefix+"internal/command/clean",
		nextPrefix+"cmd/wtm-next",
		nextPrefix+"internal/stray",
	)
}

func TestANextPackageKeepsItsCommentsUnderItsCeiling(t *testing.T) {
	ceilings := map[string]int{
		"internal/domain/dotenv":  2,
		"internal/domain/envcopy": 1,
		"internal/adapter/shell":  1,
		"internal/adapter/oplock": 2,
	}
	runTxtar(t, newNextCommentsAnalyzer(nextCommentsParams{Tests: loadedTests, Ceilings: func() (map[string]int, error) { return ceilings, nil }}), "nextcomments",
		nextPrefix+"internal/domain/branch",
		nextPrefix+"internal/domain/dotenv",
		nextPrefix+"internal/domain/envcopy",
		nextPrefix+"internal/adapter/shell",
		nextPrefix+"internal/adapter/oplock",
	)
}

func TestTheCeilingsFileReadsOnePackageAndItsCountPerLine(t *testing.T) {
	ceilings, err := parseNextCeilings("# why each one earns its place: review\n\ninternal/adapter/shell 3\ninternal/domain/dotenv   1\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]int{"internal/adapter/shell": 3, "internal/domain/dotenv": 1}; !maps.Equal(ceilings, want) {
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
				nextPrefix+"internal/ui/output",
			)
		})
	}
}

func TestALanguageRuleStillActsOnNext(t *testing.T) {
	runTxtar(t, typeassertAnalyzer, "nextlegacy", nextPrefix+"internal/domain/assert")
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
