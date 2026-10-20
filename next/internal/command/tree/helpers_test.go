package tree_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/stretchr/testify/require"
)

// subjects are the subjects of read's items, in order.
func subjects(read tree.Outcome) []string {
	var names []string
	for _, item := range read.Items {
		names = append(names, item.Subject)
	}
	return names
}

// detailOf is the detail of the item of read whose subject is subject.
func detailOf(t *testing.T, read tree.Outcome, subject string) tree.Detail {
	t.Helper()
	for _, item := range read.Items {
		if item.Subject == subject {
			require.NotNil(t, item.Detail, subject)
			return *item.Detail
		}
	}
	t.Fatalf("no item %s in %v", subject, subjects(read))
	return tree.Detail{}
}

// codes are the codes of read's warnings, in order.
func codes(read tree.Outcome) []string {
	var found []string
	for _, warning := range read.Warnings {
		found = append(found, warning.Code)
	}
	return found
}

// count is n as a count of commits.
func count(n int) *int {
	return &n
}
