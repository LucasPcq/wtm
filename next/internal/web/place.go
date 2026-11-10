package web

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// Located is what a path is a part of: its repository, and whether wtm is set up in it.
type Located struct {
	Repo  domain.Repo
	SetUp bool
}

// A Place is a path as POST /api/repos/check reads it: where it stands for wtm, and, in a repository, that repository's id, its main worktree and its name.
type Place struct {
	Path  string     `json:"path"`
	State PlaceState `json:"state"`
	ID    string     `json:"id,omitempty"`
	Root  string     `json:"root,omitempty"`
	Name  string     `json:"name,omitempty"`
}

// PlaceState is where a path stands for wtm.
type PlaceState string

// The states of a path.
const (
	PlaceMissing       PlaceState = "missing"          // nothing is there
	PlaceNotRepository PlaceState = "not_a_repository" // a directory outside any git repository
	PlaceNotSetUp      PlaceState = "not_set_up"       // in a git repository wtm is not set up in
	PlaceSetUp         PlaceState = "set_up"           // in a repository set up, which the web does not list
	PlaceKnown         PlaceState = "known"            // in a repository the web lists already
)

// A placeCall is the body of POST /api/repos/check and POST /api/repos: the path, absolute, or from the home as ~/….
type placeCall struct {
	Path string `json:"path"`
}

// CodePlaceRefused refuses to add a path that is in no git repository.
const CodePlaceRefused = "web.place_refused"

// check answers where a path stands, changing nothing.
func (s *Server) check(w http.ResponseWriter, r *http.Request) {
	place, _, ok := s.place(w, r)
	if ok {
		writeJSON(w, http.StatusOK, place)
	}
}

// add lists the repository of a path: one set up joins the repositories wtm knows, one not set up is served until the server stops, so that it can be set up; a path in none is refused.
func (s *Server) add(w http.ResponseWriter, r *http.Request) {
	place, located, ok := s.place(w, r)
	if !ok {
		return
	}

	switch place.State {
	case PlaceMissing, PlaceNotRepository:
		writeError(w, http.StatusBadRequest, &domain.Error{Kind: domain.KindInvalid, Code: CodePlaceRefused, Message: place.Path + " is in no git repository", Params: domain.Params{"path": place.Path, "state": string(place.State)}})
		return
	case PlaceSetUp:
		s.params.Register(r.Context(), located.Repo)
		place.State = PlaceKnown
	case PlaceNotSetUp:
		s.mu.Lock()
		s.pending[place.ID] = located.Repo
		s.mu.Unlock()
	case PlaceKnown:
	}
	writeJSON(w, http.StatusOK, place)
}

// place reads the path of the body and where it stands, or writes its refusal and reports false.
func (s *Server) place(w http.ResponseWriter, r *http.Request) (Place, Located, bool) {
	var call placeCall
	err := decode(w, r, &call)
	if home := s.params.Home; home != "" && (call.Path == "~" || strings.HasPrefix(call.Path, "~/")) {
		call.Path = filepath.Join(home, strings.TrimPrefix(call.Path, "~"))
	}
	if err != nil || !filepath.IsAbs(call.Path) {
		writeError(w, http.StatusBadRequest, domain.InvalidMessage("the body of the request names no absolute path"))
		return Place{}, Located{}, false
	}

	place, located, err := s.placeOf(r.Context(), filepath.Clean(call.Path))
	if err != nil {
		writeError(w, http.StatusInternalServerError, domain.AsError(domain.AsErrorParams{Err: err}))
		return Place{}, Located{}, false
	}
	return place, located, true
}

// placeOf is where path stands: missing, in no repository, or in one, set up or not, listed or not.
func (s *Server) placeOf(ctx context.Context, path string) (Place, Located, error) {
	place := Place{Path: path}
	located, err := s.params.Locate(ctx, path)
	if failure, ok := errors.AsType[*domain.Error](err); ok && failure.Code == domain.CodeNotARepository {
		place.State = PlaceNotRepository
		return place, Located{}, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		place.State = PlaceMissing
		return place, Located{}, nil
	}
	if err != nil {
		return Place{}, Located{}, err
	}

	place.ID, place.Root, place.Name = RepoID(located.Repo), located.Repo.Root, filepath.Base(located.Repo.Root)
	switch known, err := s.known(ctx, place.ID); {
	case err != nil:
		return Place{}, Located{}, err
	case known:
		place.State = PlaceKnown
	case located.SetUp:
		place.State = PlaceSetUp
	default:
		place.State = PlaceNotSetUp
	}
	return place, located, nil
}

// known reports whether the repository of id is one the web lists as set up.
func (s *Server) known(ctx context.Context, id string) (bool, error) {
	repos, err := s.params.Repos(ctx)
	if err != nil {
		return false, err
	}
	for _, repo := range repos {
		if RepoID(repo) == id {
			return true, nil
		}
	}
	return false, nil
}
