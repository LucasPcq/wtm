package owed

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/flow"
)

func TestHoldingsOfNothingReadsNothing(t *testing.T) {
	var holdings Holdings
	if held := holdings.Of(t.Context(), flow.Context{StateDir: t.TempDir()}, nil).Held(); len(held) != 0 {
		t.Errorf("held = %v, want nothing for an empty selection", held)
	}
}

func TestHoldingsWithoutASharedServiceHoldNothing(t *testing.T) {
	var holdings Holdings
	if held := holdings.Of(t.Context(), flow.Context{StateDir: t.TempDir()}, []string{"feat"}).Held(); len(held) != 0 {
		t.Errorf("held = %v, want none", held)
	}
}
