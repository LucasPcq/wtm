package resolve_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/require"
)

// resolved runs req over w and requires the one worktree it resolved.
func resolved(t *testing.T, w world, req resolve.Request) resolve.Detail {
	t.Helper()
	got, err := w.usecase.Run(t.Context(), req, domain.RunOptions{})
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.NotNil(t, got.Items[0].Detail, got.Items[0].Err)
	return *got.Items[0].Detail
}

// selectOf is the worktree field of q.
func selectOf(t *testing.T, q resolve.Questions) domain.SelectField {
	t.Helper()
	field, ok := q.Form().Fields[0].(domain.SelectField)
	require.True(t, ok)
	return field
}
