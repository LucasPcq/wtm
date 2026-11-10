package envshow_test

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/envshow"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/require"
)

func read(t *testing.T, s seed, req envshow.Request) (envshow.Detail, []domain.Warning) {
	t.Helper()
	outcome, err := newUseCase(s).Run(t.Context(), req, domain.RunOptions{})
	require.NoError(t, err)
	require.Len(t, outcome.Items, 1)
	return *outcome.Items[0].Detail, outcome.Warnings
}

func keyOf(t *testing.T, d envshow.Detail, file, key string) envshow.Key {
	t.Helper()
	for _, app := range d.Apps {
		for _, f := range app.Files {
			if f.Target != file {
				continue
			}
			at := slices.IndexFunc(f.Keys, func(k envshow.Key) bool { return k.Key == key })
			require.GreaterOrEqual(t, at, 0, key)
			return f.Keys[at]
		}
	}
	require.Fail(t, "no file "+file)
	return envshow.Key{}
}

func ptr(value string) *string {
	return &value
}
