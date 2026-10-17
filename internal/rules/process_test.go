package rules

import (
	"slices"
	"testing"
)

const processTable = `  1     0     1
 10     1   100
 20    10   100
 21    10   100
 30    20   100
 31    20   310
 32    31   310
 40     1   100
bogus line
`

func TestDescendantsWalksTheTreeBelowTheRootWithinItsGroup(t *testing.T) {
	links := ParseProcessLinks(processTable)

	got := Descendants(DescendantsParams{Links: links, Root: 10, Group: 100})

	if want := []int{20, 21, 30}; !slices.Equal(got, want) {
		t.Errorf("descendants = %v, want %v — 31 daemonized, and what it started is its own", got, want)
	}
	if got := Descendants(DescendantsParams{Links: links, Root: 30, Group: 100}); len(got) != 0 {
		t.Errorf("a leaf has descendants %v", got)
	}
}

func TestStillInGroupDropsWhatDaemonizedOrDied(t *testing.T) {
	links := ParseProcessLinks(processTable)

	got := StillInGroup(StillInGroupParams{Links: links, PIDs: []int{20, 31, 99}, Group: 100})

	if want := []int{20}; !slices.Equal(got, want) {
		t.Errorf("kept = %v, want %v", got, want)
	}
}
