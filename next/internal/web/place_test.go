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
		"not_a_folder":     {"/tmp/notes.txt", Place{Path: "/tmp/notes.txt", State: PlaceNotFolder}},
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
	assert.Empty(t, located.registered)
	assert.Len(t, decoded[[]Repo](t, do(t, server, req{method: http.MethodGet, path: "/api/repos"})), 1, "nothing left waiting to be set up")
}

// Review of #286: a register that did not land (its timeout, a state dir moved) left a repository set up that no route served.
func TestARepositorySetUpWhoseRegisterDidNotLandIsServedAsSetUp(t *testing.T) {
	located := &places{}
	server := serving(t, Params{Locate: located.locate, Register: located.register})

	added := do(t, server, req{method: http.MethodPost, path: "/api/repos", body: `{"path":"/code/other"}`})
	commands := do(t, server, req{method: http.MethodGet, path: "/api/repos/" + RepoID(other) + "/commands"})

	assert.Equal(t, PlaceKnown, decoded[Place](t, added).State)
	assert.Equal(t, http.StatusOK, commands.StatusCode)
	assert.Contains(t, decoded[[]Repo](t, do(t, server, req{method: http.MethodGet, path: "/api/repos"})), Repo{ID: RepoID(other), Root: other.Root, CommonDir: other.CommonDir, SetUp: true})
}

// Review of #286: a repository added then set up read not set up until the registry listed it, and its gate came back.
func TestARepositoryAddedReadsSetUpOnceItsSettingsAreWritten(t *testing.T) {
	located := &places{}
	server := serving(t, Params{Locate: located.locate, Register: located.register})
	do(t, server, req{method: http.MethodPost, path: "/api/repos", body: `{"path":"/code/fresh"}`})

	located.mu.Lock()
	located.written = true
	located.mu.Unlock()

	assert.Contains(t, decoded[[]Repo](t, do(t, server, req{method: http.MethodGet, path: "/api/repos"})), Repo{ID: RepoID(fresh), Root: fresh.Root, CommonDir: fresh.CommonDir, SetUp: true})
}

// Review of #286: only the front kept a repository not set up to init; every route ran any command on it.
func TestARepositoryNotSetUpTakesOnlyTheCommandsBeforeASetUp(t *testing.T) {
	located := &places{}
	server := serving(t, Params{Locate: located.locate, Register: located.register, NotSetUp: []string{"init"}})
	do(t, server, req{method: http.MethodPost, path: "/api/repos", body: `{"path":"/code/fresh"}`})
	greet := "/api/repos/" + RepoID(fresh) + "/commands/greet"

	asked := do(t, server, req{method: http.MethodPost, path: greet + "/questions", body: `{}`})
	ran := do(t, server, req{method: http.MethodPost, path: greet, body: `{}`})
	located.mu.Lock()
	located.written = true
	located.mu.Unlock()
	setUp := do(t, server, req{method: http.MethodPost, path: greet + "/questions", body: `{}`})

	for _, refused := range []*http.Response{asked, ran} {
		assert.Equal(t, http.StatusPreconditionFailed, refused.StatusCode)
		assert.Equal(t, CodeNotSetUp, decoded[envelope](t, refused).Error.Code)
	}
	assert.Equal(t, http.StatusOK, setUp.StatusCode)
}

func TestAPathTypedFromTheHomeIsReadFromIt(t *testing.T) {
	located := &places{}
	server := serving(t, Params{Locate: located.locate, Register: located.register, Home: "/code"})

	response := do(t, server, req{method: http.MethodPost, path: "/api/repos/check", body: `{"path":"~/fresh/src"}`})

	require.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, Place{Path: "/code/fresh/src", State: PlaceNotSetUp, ID: RepoID(fresh), Root: fresh.Root, Name: "fresh"}, decoded[Place](t, response))
}
