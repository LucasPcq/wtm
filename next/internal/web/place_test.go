package web

import (
	"net/http"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPathIsCheckedForWhereItStandsChangingNothing(t *testing.T) {
	located := &places{}
	server := serving(t, Params{Locate: located.locate, Register: located.register})
	cases := map[string]struct {
		path string
		want Place
	}{
		"missing":          {"/missing/app", Place{Path: "/missing/app", State: PlaceMissing}},
		"not_a_repository": {"/tmp/notes", Place{Path: "/tmp/notes", State: PlaceNotRepository}},
		"not_set_up":       {"/code/fresh/src", Place{Path: "/code/fresh/src", State: PlaceNotSetUp, ID: RepoID(fresh), Root: fresh.Root, Name: "fresh"}},
		"set_up":           {"/code/other", Place{Path: "/code/other", State: PlaceSetUp, ID: RepoID(other), Root: other.Root, Name: "other"}},
		"known":            {"/code/app", Place{Path: "/code/app", State: PlaceKnown, ID: RepoID(repo), Root: repo.Root, Name: "app"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			response := do(t, server, req{method: http.MethodPost, path: "/api/repos/check", body: `{"path":"` + c.path + `"}`})

			require.Equal(t, http.StatusOK, response.StatusCode)
			assert.Equal(t, c.want, decoded[Place](t, response))
		})
	}
	assert.Empty(t, located.registered)
	assert.Len(t, decoded[[]Repo](t, do(t, server, req{method: http.MethodGet, path: "/api/repos"})), 1)
}

func TestARepositoryNotSetUpIsServedOnceAdded(t *testing.T) {
	located := &places{}
	server := serving(t, Params{Locate: located.locate, Register: located.register})

	added := do(t, server, req{method: http.MethodPost, path: "/api/repos", body: `{"path":"/code/fresh"}`})
	commands := do(t, server, req{method: http.MethodGet, path: "/api/repos/" + RepoID(fresh) + "/commands"})

	require.Equal(t, http.StatusOK, added.StatusCode)
	assert.Equal(t, PlaceNotSetUp, decoded[Place](t, added).State)
	assert.Equal(t, http.StatusOK, commands.StatusCode)
	assert.Equal(t, []Repo{
		{ID: RepoID(repo), Root: repo.Root, CommonDir: repo.CommonDir, SetUp: true},
		{ID: RepoID(fresh), Root: fresh.Root, CommonDir: fresh.CommonDir},
	}, decoded[[]Repo](t, do(t, server, req{method: http.MethodGet, path: "/api/repos"})))
	assert.Empty(t, located.registered)
}

func TestARepositorySetUpElsewhereIsRegisteredAsItIs(t *testing.T) {
	located := &places{}
	server := serving(t, Params{Locate: located.locate, Register: located.register})

	added := do(t, server, req{method: http.MethodPost, path: "/api/repos", body: `{"path":"/code/other"}`})

	require.Equal(t, http.StatusOK, added.StatusCode)
	assert.Equal(t, PlaceKnown, decoded[Place](t, added).State)
	assert.Equal(t, []domain.Repo{other}, located.registered)
}

func TestAPathInNoRepositoryIsNotAdded(t *testing.T) {
	located := &places{}
	server := serving(t, Params{Locate: located.locate, Register: located.register})

	for _, body := range []string{`{"path":"/tmp/notes"}`, `{"path":"/missing"}`, `{"path":"relative"}`} {
		response := do(t, server, req{method: http.MethodPost, path: "/api/repos", body: body})

		assert.Equal(t, http.StatusBadRequest, response.StatusCode, body)
	}
}

func TestAPathTypedFromTheHomeIsReadFromIt(t *testing.T) {
	located := &places{}
	server := serving(t, Params{Locate: located.locate, Register: located.register, Home: "/code"})

	response := do(t, server, req{method: http.MethodPost, path: "/api/repos/check", body: `{"path":"~/fresh/src"}`})

	require.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, Place{Path: "/code/fresh/src", State: PlaceNotSetUp, ID: RepoID(fresh), Root: fresh.Root, Name: "fresh"}, decoded[Place](t, response))
}
