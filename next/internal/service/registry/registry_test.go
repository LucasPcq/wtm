package registry_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/registrytest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARepositoryRegisteredIsAnnouncedOnce(t *testing.T) {
	store, b := registrytest.NewFake(registrytest.FakeParams{}), &bus{}
	s := service(store, initialized(main), b)

	s.Register(t.Context(), main)
	s.Register(t.Context(), main)

	assert.Equal(t, []domain.Repo{main}, store.Repos())
	assert.Equal(t, []announced{{Event: domain.RepoAdded{}, Repo: main}}, b.announcements())
}

func TestARepositoryWithNoWtmConfigIsNeverListed(t *testing.T) {
	store, b := registrytest.NewFake(registrytest.FakeParams{}), &bus{}

	service(store, initialized(), b).Register(t.Context(), main)

	assert.Empty(t, store.Calls())
	assert.Empty(t, b.announcements())
}

func TestARepositoryAddedPrunesThoseNoLongerInitialized(t *testing.T) {
	store, b := registrytest.NewFake(registrytest.FakeParams{Repos: []domain.Repo{other}}), &bus{}

	service(store, initialized(main), b).Register(t.Context(), main)

	assert.Equal(t, []domain.Repo{main}, store.Repos())
	assert.Equal(t, []announced{{Event: domain.RepoAdded{}, Repo: main}, {Event: domain.RepoRemoved{}, Repo: other}}, b.announcements())
}

func TestPruneAnnouncesEachRepositoryItDropsAndReturnsTheOthers(t *testing.T) {
	store, b := registrytest.NewFake(registrytest.FakeParams{Repos: []domain.Repo{main, other}}), &bus{}

	kept, err := service(store, initialized(main), b).Prune(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []domain.Repo{main}, kept)
	assert.Equal(t, []announced{{Event: domain.RepoRemoved{}, Repo: other}}, b.announcements())
}

func TestARegistrationTheStoreRefusesIsDroppedSilently(t *testing.T) {
	store, b := registrytest.NewFake(registrytest.FakeParams{Fail: map[string]error{"Add": errors.New("disk full")}}), &bus{}

	service(store, initialized(main), b).Register(t.Context(), main)

	assert.Empty(t, store.Repos())
	assert.Empty(t, b.announcements())
}

func TestAMovedStateDirectoryLeavesTheRegistryAsItIs(t *testing.T) {
	store, b := registrytest.NewFake(registrytest.FakeParams{Repos: []domain.Repo{other}}), &bus{}
	moved := initialized(main)
	moved.moved = "/sandbox/state"
	moved.configs["/sandbox/state"] = true
	s := service(store, moved, b)

	s.Register(t.Context(), main)
	kept, err := s.Prune(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []domain.Repo{other}, kept)
	assert.Equal(t, []domain.Repo{other}, store.Repos())
	assert.Empty(t, b.announcements())
}
