package repository_test

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"

	"github.com/stretchr/testify/require"
)

func offerOf(t *testing.T, offers []repository.Offer, action worktree.Action) repository.Offer {
	t.Helper()
	at := slices.IndexFunc(offers, func(offer repository.Offer) bool { return offer.Action == action })
	require.GreaterOrEqual(t, at, 0, action)
	return offers[at]
}
