package main

import (
	"os"
	"strings"
	"testing"
)

func TestANewMigratingEntryIsOverTheCeiling(t *testing.T) {
	list, err := parseMigrating("statewrite internal/service/probe/probe\\.go 1\n")
	if err != nil {
		t.Fatal(err)
	}
	if over := overCeiling(list, nil); len(over) != 1 || !strings.Contains(over[0], "is a new entry") {
		t.Errorf("over = %q, want the new entry refused", over)
	}
}

func TestARaisedCountIsOverTheCeiling(t *testing.T) {
	list, err := parseMigrating("statewrite internal/service/execsvc/execsvc\\.go 2\n")
	if err != nil {
		t.Fatal(err)
	}
	if over := overCeiling(list, nil); len(over) != 1 || !strings.Contains(over[0], "raises its count above 1") {
		t.Errorf("over = %q, want the raised count refused", over)
	}
}

func TestAShrunkListNeedsNoEditOfTheCeiling(t *testing.T) {
	list, err := parseMigrating("stateowner internal/config/write\\.go 3\n")
	if err != nil {
		t.Fatal(err)
	}
	if over := overCeiling(list, []string{`TextListModel\.helpRowless`}); len(over) != 0 {
		t.Errorf("over = %q, want a smaller list accepted", over)
	}
}

func TestANewDeadcodePatternIsOverTheCeiling(t *testing.T) {
	over := overCeiling(migratingList{}, parseDeadcodeIgnore("# reason\nNewThing\\.unused\n"))
	if len(over) != 1 || !strings.Contains(over[0], "is a new pattern") {
		t.Errorf("over = %q, want the new pattern refused", over)
	}
}

func TestTheExceptionFilesAreWithinTheirCeilings(t *testing.T) {
	t.Chdir("../..")
	data, err := os.ReadFile(".archlint-migrating")
	if err != nil {
		t.Fatal(err)
	}
	list, err := parseMigrating(string(data))
	if err != nil {
		t.Fatal(err)
	}
	deadcode, err := loadDeadcodeIgnore()
	if err != nil {
		t.Fatal(err)
	}
	if over := overCeiling(list, deadcode); len(over) != 0 {
		t.Errorf("the exception files are over their ceilings:\n%s", strings.Join(over, "\n"))
	}
}
