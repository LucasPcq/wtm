package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/rules"
)

func TestClassifyIndexVersionReadsItsOwnFormat(t *testing.T) {
	access := rules.ClassifyIndexVersion(rules.IndexVersionParams{File: 2, Binary: 2})

	if !access.Read || access.Freeze {
		t.Fatalf("access = %+v, want the entries read and nothing frozen", access)
	}
}

func TestClassifyIndexVersionReplacesAnAbandonedFormat(t *testing.T) {
	access := rules.ClassifyIndexVersion(rules.IndexVersionParams{File: 1, Binary: 2})

	if access.Read {
		t.Fatal("an older format holds nothing this binary can read")
	}
	if access.Freeze {
		t.Fatal("freezing on a stale file disables the index outright: nothing is recorded and nothing is ever picked back up")
	}
}

func TestClassifyIndexVersionFreezesOnANewerBinarysIndex(t *testing.T) {
	access := rules.ClassifyIndexVersion(rules.IndexVersionParams{File: 3, Binary: 2})

	if !access.Freeze {
		t.Fatal("overwriting a newer binary's index would destroy its record of what it started")
	}
	if access.Read {
		t.Fatal("a format from the future is not one this binary can read")
	}
}
