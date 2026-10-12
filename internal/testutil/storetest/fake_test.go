package storetest_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/store"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

// The fake's agreement with the disk is tested in internal/store
// (contract_test.go); here, what only the fake does.

func TestAFakeMethodGivenAnErrorFailsWithoutRunning(t *testing.T) {
	full := errors.New("disk full")
	fake := storetest.NewFake(storetest.FakeParams{
		Config: &domain.ProjectConfig{},
		Fail:   map[string]error{"Update": full, "Remember": full, "Edit": full},
	})

	updateErr := fake.Update(t.Context(), store.MetaUpdate{Branch: "feat/a", Change: func(*domain.WorktreeMetadata) error { return nil }})
	rememberErr := fake.Remember(t.Context(), rules.RememberedChange{Remember: map[string]string{domain.RememberIsolation: "verbatim"}})
	_, editErr := fake.Edit(t.Context(), store.EnvEdit{Path: "/wt/.env", Create: true, Edit: func(lines []domain.EnvLine) []domain.EnvLine { return lines }})

	require.ErrorIs(t, updateErr, full)
	require.ErrorIs(t, rememberErr, full)
	require.ErrorIs(t, editErr, full)
	records, err := fake.Records()
	require.NoError(t, err)
	assert.Empty(t, records)
	config, ok := fake.Config()
	require.True(t, ok)
	assert.Empty(t, config.Wizard.Remembered)
	_, exists := fake.File("/wt/.env")
	assert.False(t, exists)
}

func TestAFakeRecordsEveryCallInOrder(t *testing.T) {
	fake := storetest.NewFake(storetest.FakeParams{})

	_, _ = fake.Load("feat/a")
	require.NoError(t, fake.Purge("feat/a"))
	_, err := fake.Copy(t.Context(), store.EnvCopy{Strategy: domain.EnvStrategyMain, TargetDir: "/wt"})
	require.NoError(t, err)

	assert.Equal(t, []string{"Load feat/a", "Purge feat/a", "Copy /wt"}, fake.Calls())
}

func TestAFakeWithoutAConfigRefusesToRemember(t *testing.T) {
	fake := storetest.NewFake(storetest.FakeParams{})

	err := fake.Remember(t.Context(), rules.RememberedChange{Remember: map[string]string{domain.RememberIsolation: "verbatim"}})

	require.ErrorIs(t, err, domain.ErrConfigNotFound)
}
