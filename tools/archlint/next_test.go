package main

import (
	"maps"
	"testing"
)

func TestANextPackageImportsOnlyWhatItsRowAllows(t *testing.T) {
	runTxtar(t, nextAnalyzer, "next",
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
		"internal/wtm/branch":     1,
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
	for _, line := range []string{"internal/adapter/shell", "internal/adapter/shell x", "internal/adapter/shell -1"} {
		if _, err := parseNextCeilings(line); err == nil {
			t.Errorf("%q parsed, want an error", line)
		}
	}
}
