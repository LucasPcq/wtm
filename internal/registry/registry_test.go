package registry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/query/operations"
	"github.com/LucasPcq/wtm/internal/registry"
	"github.com/LucasPcq/wtm/internal/testutil/dispatchtest"
)

func TestTheProductionRegistryServesTheQueryOfTheOperationsInProgress(t *testing.T) {
	production := registry.Production(dispatchtest.New(t, dispatchtest.BenchParams{}).Dispatcher)

	entry, found := production.Lookup(operations.Name)

	require.True(t, found)
	reply, err := entry.Call(t.Context(), dispatch.Call{})
	require.NoError(t, err)
	view, ok := reply.(dispatch.QueryReply)
	require.True(t, ok, "got %T", reply)
	assert.JSONEq(t, `{"operations":[]}`, string(view.View))
}

func TestTheProductionCatalogueBuilds(t *testing.T) {
	production := registry.Production(dispatchtest.New(t, dispatchtest.BenchParams{}).Dispatcher)

	assert.NotPanics(t, func() { production.Catalog() })
}
