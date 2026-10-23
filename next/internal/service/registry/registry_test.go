package registry_test

import (
	"context"
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

func TestAMovedStateDirectoryLeavesTheRegistryAloneAndListsNothing(t *testing.T) {
	store, b := registrytest.NewFake(registrytest.FakeParams{Repos: []domain.Repo{other}}), &bus{}
	moved := initialized(main, other)
	moved.moved = true
	s := service(store, moved, b)

	s.Register(t.Context(), main)
	kept, err := s.Prune(t.Context())

	require.NoError(t, err)
	assert.Empty(t, kept)
	assert.Equal(t, []domain.Repo{other}, store.Repos())
	assert.Empty(t, store.Calls())
	assert.Empty(t, b.announcements())
}

func TestWhatTheStoreCommittedIsAnnouncedOnceTheCallerIsGone(t *testing.T) {
	store, b := registrytest.NewFake(registrytest.FakeParams{Repos: []domain.Repo{other}}), &bus{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	s := service(store, initialized(main), b)

	s.Register(ctx, main)

	assert.Equal(t, []string{"Add /code/app/.git"}, store.Calls(), "no prune once the caller is gone")
	assert.Equal(t, []announced{{Event: domain.RepoAdded{}, Repo: main}}, b.announcements())

	kept, err := s.Prune(ctx)

	require.NoError(t, err)
	assert.Equal(t, []domain.Repo{main}, kept)
	assert.Equal(t, []announced{{Event: domain.RepoAdded{}, Repo: main}, {Event: domain.RepoRemoved{}, Repo: other}}, b.announcements())
}
