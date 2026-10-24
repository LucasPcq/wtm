// Package web serves the catalogue of wtm-next's commands and its event stream over local HTTP to a browser: the routes of each repository wtm knows, the guard of every request (a loopback host, the page's own origin, the token of this start), an operation's progress and the events as server-sent events, and the page.
package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

//go:embed page.html
var page []byte

// Params are what a server serves and the functions it is given in place of the world it cannot reach: the catalogue and the event stream, the repositories wtm knows, the version of the binary, the token every request carries, the kill of a forced cancel, the command line of a follow-up and the published schema.
type Params struct {
	Catalogue api.Catalogue
	Events    api.Stream
	// Repos lists the repositories wtm knows, dropping those no longer initialized.
	Repos   func(ctx context.Context) ([]domain.Repo, error)
	Version string
	Token   string
	// Kill stops at once the children of every operation of the process already cancelled, as a second Ctrl+C does: it is process-wide, never one operation's.
	Kill func()
	// CommandLine spells a follow-up as the command line runs it, for one the catalogue cannot run.
	CommandLine func(followUp domain.FollowUp) []string
	// Schema returns the published schema of the API; nil while none is published.
	Schema func() []byte
	// Idle is how long the server waits with no request before it stops.
	Idle time.Duration
}

// A Server is the HTTP surface of wtm-next on one machine. Its zero value is not usable: build it with New.
type Server struct {
	params Params
	mux    *http.ServeMux

	// stopping is done once the server stops: the event streams end with it, the operations never do.
	stopping context.Context
	stop     context.CancelFunc
	// asked is closed once a client asks the server to stop (POST /api/stop).
	asked     chan struct{}
	askedOnce sync.Once

	mu        sync.Mutex
	running   map[string]running // by correlation id
	active    int
	idleSince time.Time
}

// running is an operation in progress: the repository it runs on and the cancel of its own context.
type running struct {
	repo   string
	cancel context.CancelFunc
}

// New returns the server of params.
func New(params Params) *Server {
	stopping, stop := context.WithCancel(context.Background())
	s := &Server{params: params, stopping: stopping, stop: stop, running: map[string]running{}, idleSince: time.Now(), asked: make(chan struct{})}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.page)
	mux.HandleFunc("GET /api/version", s.version)
	mux.HandleFunc("GET /api/repos", s.repos)
	mux.HandleFunc("GET /api/repos/{repo}/commands", s.commands)
	mux.HandleFunc("POST /api/repos/{repo}/commands/{name}/questions", s.questions)
	mux.HandleFunc("POST /api/repos/{repo}/commands/{name}", s.execute)
	mux.HandleFunc("DELETE /api/repos/{repo}/operations/{id}", s.cancel)
	mux.HandleFunc("GET /api/events", s.events)
	mux.HandleFunc("GET /api/schema", s.schema)
	mux.HandleFunc("POST /api/stop", s.stopAsked)
	mux.HandleFunc("POST /api/command-line", s.commandLine)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, &domain.Error{Kind: domain.KindInvalid, Code: CodeNotFound, Message: "no such route"})
	})
	s.mux = mux
	return s
}

// ServeHTTP guards r, then serves it, counting it among the requests in flight.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.guard(w, r) {
		return
	}

	s.begin()
	defer s.end()
	s.mux.ServeHTTP(w, r)
}

// Serve serves on listener until ctx is done, a client asks it to stop, or the server stays Idle with no request in flight. It then stops taking requests, ends the event streams and waits for the operations still running, which finish whatever happens.
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	if s.params.Idle <= 0 {
		return fmt.Errorf("a server that stops once idle %s would stop before its first request", s.params.Idle)
	}
	server := &http.Server{Handler: s, ReadHeaderTimeout: readHeaderTimeout}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()

	ticker := time.NewTicker(max(min(s.params.Idle/idleChecks, time.Minute), time.Millisecond))
	defer ticker.Stop()
wait:
	for {
		select {
		case <-ctx.Done():
			break wait
		case <-s.asked:
			break wait
		case err := <-served:
			s.stop()
			return err
		case <-ticker.C:
			if s.idle() {
				break wait
			}
		}
	}

	s.stop()
	err := server.Shutdown(context.WithoutCancel(ctx))
	if served := <-served; !errors.Is(served, http.ErrServerClosed) {
		return served
	}
	return err
}

const (
	readHeaderTimeout = 10 * time.Second
	// idleChecks is how many times per Idle the server checks whether it is idle.
	idleChecks = 10
)

func (s *Server) begin() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active++
}

func (s *Server) end() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active--
	if s.active == 0 {
		s.idleSince = time.Now()
	}
}

// idle reports whether the server has had no request in flight for Idle; an operation runs within its request, an open event stream is one.
func (s *Server) idle() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active == 0 && time.Since(s.idleSince) >= s.params.Idle
}

// ----------------------------------------------------------------------------
// Repositories

// A Repo is a repository as the routes name it: its ID, the {repo} of a route, and where it is.
type Repo struct {
	ID        string `json:"id"`
	Root      string `json:"root"`
	CommonDir string `json:"common_dir"`
}

// repoIDBytes is how many bytes of the SHA-256 of a repository's common directory its ID keeps.
const repoIDBytes = 6

// RepoID returns the ID of repo in the routes: a digest of its common directory, its key on the event bus, the same from any of its worktrees.
func RepoID(repo domain.Repo) string {
	sum := sha256.Sum256([]byte(repo.CommonDir))
	return hex.EncodeToString(sum[:repoIDBytes])
}

// repo returns the repository of the {repo} of r, or writes its refusal and reports false.
func (s *Server) repo(w http.ResponseWriter, r *http.Request) (domain.Repo, bool) {
	return s.repoOf(w, r, r.PathValue("repo"))
}

// repoOf returns the repository whose ID is id, or writes its refusal and reports false.
func (s *Server) repoOf(w http.ResponseWriter, r *http.Request, id string) (domain.Repo, bool) {
	repos, err := s.params.Repos(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, domain.AsError(domain.AsErrorParams{Err: err}))
		return domain.Repo{}, false
	}

	for _, repo := range repos {
		if RepoID(repo) == id {
			return repo, true
		}
	}
	writeError(w, http.StatusNotFound, &domain.Error{Kind: domain.KindInvalid, Code: CodeRepoUnknown, Message: "no repository wtm knows has the id " + id})
	return domain.Repo{}, false
}

// ----------------------------------------------------------------------------
// Routes

func (s *Server) page(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(page)
}

func (s *Server) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.params.Version})
}

func (s *Server) repos(w http.ResponseWriter, r *http.Request) {
	repos, err := s.params.Repos(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, domain.AsError(domain.AsErrorParams{Err: err}))
		return
	}

	listed := make([]Repo, 0, len(repos))
	for _, repo := range repos {
		listed = append(listed, Repo{ID: RepoID(repo), Root: repo.Root, CommonDir: repo.CommonDir})
	}
	writeJSON(w, http.StatusOK, listed)
}

// A listedCommand is a command of the catalogue as GET …/commands lists it: its name and its form outside any repository.
type listedCommand struct {
	Name string      `json:"name"`
	Form domain.Form `json:"form"`
}

func (s *Server) commands(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.repo(w, r); !ok {
		return
	}

	listed := make([]listedCommand, 0, len(s.params.Catalogue))
	for _, command := range s.params.Catalogue {
		listed = append(listed, listedCommand{Name: command.Name, Form: command.Form})
	}
	writeJSON(w, http.StatusOK, listed)
}

// A body is what a POST of a command carries: the answer of each field by its path, and, to run it, whether it is a dry run and the fingerprint its apply is held to.
type body struct {
	Answers           []domain.Answered `json:"answers"`
	DryRun            bool              `json:"dry_run"`
	ExpectFingerprint string            `json:"expect_fingerprint"`
}

func (s *Server) questions(w http.ResponseWriter, r *http.Request) {
	repo, command, call, ok := s.call(w, r)
	if !ok {
		return
	}

	form, err := command.Questions(r.Context(), api.Call{Dir: repo.Root, Answers: call.Answers})
	if err != nil {
		writeEnvelope(w, domain.Envelope{Command: command.Name, Err: domain.AsError(domain.AsErrorParams{Err: err})})
		return
	}
	writeJSON(w, http.StatusOK, form)
}

// call returns the repository, the command and the body of a POST of a command, or writes its refusal and reports false.
func (s *Server) call(w http.ResponseWriter, r *http.Request) (domain.Repo, api.Command, body, bool) {
	repo, ok := s.repo(w, r)
	if !ok {
		return domain.Repo{}, api.Command{}, body{}, false
	}
	command, found := s.params.Catalogue.Find(r.PathValue("name"))
	if !found {
		writeError(w, http.StatusNotFound, &domain.Error{Kind: domain.KindInvalid, Code: CodeCommandUnknown, Message: "the catalogue has no command " + r.PathValue("name")})
		return domain.Repo{}, api.Command{}, body{}, false
	}

	var call body
	if err := decode(w, r, &call); err != nil {
		writeError(w, http.StatusBadRequest, domain.InvalidMessage("the body of the request is not a call: "+err.Error()))
		return domain.Repo{}, api.Command{}, body{}, false
	}
	return repo, command, call, true
}

// surface is the surface the operations a server runs record.
const surface = "web"

// HeaderOperation is the response header that names the operation a POST of a command started: its correlation ID, the {id} that cancels it and the correlation_id of its events.
const HeaderOperation = "X-Wtm-Operation"

// execute runs a command on the context of its operation, never the request's: a client that leaves cancels nothing, DELETE …/operations/{id} does. With Accept: text/event-stream, each event of the operation goes out as a line of the event stream as it happens, then its envelope, the event named envelope; otherwise the envelope alone, with the status of its error's kind.
func (s *Server) execute(w http.ResponseWriter, r *http.Request) {
	repo, command, call, ok := s.call(w, r)
	if !ok {
		return
	}

	id := newID()
	ctx, cancel := context.WithCancel(context.WithoutCancel(r.Context()))
	defer cancel()
	s.track(id, running{repo: RepoID(repo), cancel: cancel})
	defer s.untrack(id)

	w.Header().Set(HeaderOperation, id)
	options := domain.RunOptions{Surface: surface, CorrelationID: id, DryRun: call.DryRun, ExpectFingerprint: call.ExpectFingerprint}
	if !acceptsEvents(r) {
		writeEnvelope(w, command.Execute(ctx, api.Call{Dir: repo.Root, Answers: call.Answers, Options: options}).Envelope)
		return
	}

	events := newEventStream(w)
	events.open()
	options.Report = func(event domain.Event) {
		events.line(domain.Line{Repo: &repo, CorrelationID: id, Event: event})
	}
	envelope := command.Execute(ctx, api.Call{Dir: repo.Root, Answers: call.Answers, Options: options}).Envelope
	events.named(eventEnvelope, envelope)
}

// cancel cancels the operation {id} on {repo}, as a first Ctrl+C does; with ?force=true it then kills the children of every cancelled operation at once, as a second does.
func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repo(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	s.mu.Lock()
	op, found := s.running[id]
	s.mu.Unlock()
	if !found || op.repo != RepoID(repo) {
		writeError(w, http.StatusNotFound, &domain.Error{Kind: domain.KindInvalid, Code: CodeOperationUnknown, Message: "no operation " + id + " runs on this repository"})
		return
	}

	op.cancel()
	if r.URL.Query().Get("force") == "true" {
		s.params.Kill()
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) track(id string, op running) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running[id] = op
}

func (s *Server) untrack(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, id)
}

// events streams the events of the repository ?repo= names, of every repository without it. Its headers go out with its first line, so that a refusal before any keeps its status; it ends when the client leaves or the server stops.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	var call api.Call
	if r.URL.Query().Has("repo") {
		repo, ok := s.repoOf(w, r, r.URL.Query().Get("repo"))
		if !ok {
			return
		}
		call.Dir = repo.Root
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stopped := context.AfterFunc(s.stopping, cancel)
	defer stopped()

	events := newEventStream(w)
	response := s.params.Events.Watch(ctx, call, api.Sink{
		Send: func(line json.RawMessage) error {
			events.open()
			return events.raw(line)
		},
		Warn: func(err error) {
			events.open()
			events.named(eventWarning, warning{Message: err.Error()})
		},
	})
	switch {
	case response.Envelope.Err == nil:
	case events.isOpen():
		events.named(eventEnvelope, response.Envelope)
	default:
		writeEnvelope(w, response.Envelope)
	}
}

// A warning is what kept the event stream from a line or a repository for a while, the data of an event named warning.
type warning struct {
	Message string `json:"message"`
}

// stopAsked stops the server as its idle stop does, once the request is answered: wtm-next ui asks a server of another version to stop rather than signal a process it read from a file. While operations run it refuses, 409 with their ids: a server stopping would keep its lock until they end, with no listener left to answer.
func (s *Server) stopAsked(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	ids := slices.Sorted(maps.Keys(s.running))
	if len(ids) == 0 {
		s.askedOnce.Do(func() { close(s.asked) })
	}
	s.mu.Unlock()

	if len(ids) > 0 {
		writeJSON(w, http.StatusConflict, Busy{Operations: ids})
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// Busy is the refusal of a stop while operations run: their correlation IDs.
type Busy struct {
	Operations []string `json:"operations"`
}

func (s *Server) schema(w http.ResponseWriter, _ *http.Request) {
	if s.params.Schema == nil {
		writeError(w, http.StatusNotFound, &domain.Error{Kind: domain.KindInvalid, Code: CodeSchemaUnavailable, Message: "this wtm publishes no schema of its API yet"})
		return
	}

	w.Header().Set("Content-Type", "application/schema+json")
	_, _ = w.Write(s.params.Schema())
}

// commandLine spells the follow-up the body carries as the command line runs it: {"argv": […]}, empty when it cannot.
func (s *Server) commandLine(w http.ResponseWriter, r *http.Request) {
	var followUp domain.FollowUp
	if err := decode(w, r, &followUp); err != nil {
		writeError(w, http.StatusBadRequest, domain.InvalidMessage("the body of the request is not a follow-up: "+err.Error()))
		return
	}

	argv := s.params.CommandLine(followUp)
	if argv == nil {
		argv = []string{}
	}
	writeJSON(w, http.StatusOK, map[string][]string{"argv": argv})
}

// ----------------------------------------------------------------------------
// Helpers

// idBytes is how many random bytes a correlation ID the server makes holds.
const idBytes = 8

func newID() string {
	raw := make([]byte, idBytes)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}
