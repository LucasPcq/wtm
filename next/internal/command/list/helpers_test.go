package list_test

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/stretchr/testify/require"
)

// subjects are the subjects of read's items, in order.
func subjects(read list.Outcome) []string {
	var names []string
	for _, item := range read.Items {
		names = append(names, item.Subject)
	}
	return names
}

// detailOf is the detail of the item of read whose subject is subject.
func detailOf(t *testing.T, read list.Outcome, subject string) list.Detail {
	t.Helper()
	for _, item := range read.Items {
		if item.Subject == subject {
			require.NotNil(t, item.Detail, subject)
			return *item.Detail
		}
	}
	t.Fatalf("no item %s in %v", subject, subjects(read))
	return list.Detail{}
}

// codes are the codes of read's warnings, in order.
func codes(read list.Outcome) []string {
	var found []string
	for _, warning := range read.Warnings {
		found = append(found, warning.Code)
	}
	return found
}

func ptr(n int) *int { return &n }

// offerOf is the offer of action among offers.
func offerOf(t *testing.T, offers []repository.Offer, action worktree.Action) repository.Offer {
	t.Helper()
	at := slices.IndexFunc(offers, func(offer repository.Offer) bool { return offer.Action == action })
	require.GreaterOrEqual(t, at, 0, action)
	return offers[at]
}
