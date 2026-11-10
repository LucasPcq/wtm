// Package web serves the catalogue of wtm-next's commands and its event stream over local HTTP to a browser: the routes of each repository wtm knows, the guard of every request (a loopback host, the page's own origin, the token of this start), an operation's progress and the events as server-sent events, and the page.
package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"mime"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

// Params are what a server serves and the functions it is given in place of the world it cannot reach: the catalogue, the event stream and the stream of a job's lines, a job's log file, the repositories wtm knows, the version of the binary, the token every request carries, the kill of a forced cancel, the command line of a follow-up and the published schema.
type Params struct {
	Catalogue api.Catalogue
	Events    api.Stream
	// Logs streams a job's lines, the call's answers naming its worktree and its job, each line one line of JSON; its zero value serves none.
	Logs api.Stream
	// LogFile returns the path of a job's log file, "" when it keeps none; nil serves none.
	LogFile func(ctx context.Context, job JobLog) (string, error)
	// DaemonLog returns the path of the log of wtm's daemon, where a daemon that did not start says why; nil serves none.
	DaemonLog func() string
	// Repos lists the repositories wtm knows, dropping those no longer initialized.
	Repos func(ctx context.Context) ([]domain.Repo, error)
	// Locate finds the repository of a path, set up or not: an error wrapping fs.ErrNotExist when nothing is there, a *domain.Error coded domain.CodeNotARepository outside any.
	Locate func(ctx context.Context, path string) (Located, error)
	// Register lists a repository set up among those wtm knows; it never fails.
	Register func(ctx context.Context, repo domain.Repo)
	// Home is the user's home, where a path typed ~/… starts; empty reads ~ as it is.
	Home    string
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
	running   map[string]running     // by correlation id
	pending   map[string]domain.Repo // the repositories not set up a client added, by id, served until set up or until the server stops
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
	s := &Server{params: params, stopping: stopping, stop: stop, running: map[string]running{}, pending: map[string]domain.Repo{}, idleSince: time.Now(), asked: make(chan struct{})}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.page)
	mux.HandleFunc("GET /api/version", s.version)
	mux.HandleFunc("GET /api/repos", s.repos)
	mux.HandleFunc("POST /api/repos", s.add)
	mux.HandleFunc("POST /api/repos/check", s.check)
	mux.HandleFunc("GET /api/repos/{repo}/commands", s.commands)
	mux.HandleFunc("POST /api/repos/{repo}/commands/{name}/questions", s.questions)
	mux.HandleFunc("POST /api/repos/{repo}/commands/{name}", s.execute)
	mux.HandleFunc("DELETE /api/repos/{repo}/operations/{id}", s.cancel)
	mux.HandleFunc("GET /api/events", s.events)
	mux.HandleFunc("GET /api/repos/{repo}/jobs/{job}/logs", s.jobLines)
	mux.HandleFunc("GET /api/repos/{repo}/jobs/{job}/log", s.jobLog)
	mux.HandleFunc("GET /api/daemon/log", s.daemonLog)
	mux.HandleFunc("GET /api/schema", s.schema)
	mux.HandleFunc("POST /api/stop", s.stopAsked)
	mux.HandleFunc("POST /api/command-line", s.commandLine)
	mux.HandleFunc("/api/", s.noRoute)
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

// A Repo is a repository as the routes name it: its ID, the {repo} of a route, where it is, and whether wtm is set up in it (one not set up is one a client added, which init sets up).
type Repo struct {
	ID        string `json:"id"`
	Root      string `json:"root"`
	CommonDir string `json:"common_dir"`
	SetUp     bool   `json:"set_up"`
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
	s.mu.Lock()
	pending, added := s.pending[id]
	s.mu.Unlock()
	if added {
		return pending, true
	}
	writeError(w, http.StatusNotFound, &domain.Error{Kind: domain.KindInvalid, Code: CodeRepoUnknown, Message: "no repository wtm knows has the id " + id})
	return domain.Repo{}, false
}

// ----------------------------------------------------------------------------
// Routes

// apiMethods are the methods the routes under /api answer.
var apiMethods = []string{http.MethodGet, http.MethodPost, http.MethodDelete}

// noRoute answers a request no route under /api takes: 405 with the methods its path answers when it has a route, 404 otherwise.
func (s *Server) noRoute(w http.ResponseWriter, r *http.Request) {
	var allowed []string
	for _, method := range apiMethods {
		// a route of the path is a pattern of that method; "/api/" is this one, "" a redirect of an unclean path
		if _, pattern := s.mux.Handler(&http.Request{Method: method, URL: r.URL, Host: r.Host}); strings.HasPrefix(pattern, method+" ") {
			allowed = append(allowed, method)
			// a GET route answers HEAD too, as Go's own 405 says
			if method == http.MethodGet {
				allowed = append(allowed, http.MethodHead)
			}
		}
	}
	if len(allowed) == 0 {
		writeError(w, http.StatusNotFound, &domain.Error{Kind: domain.KindInvalid, Code: CodeNotFound, Message: "no such route"})
		return
	}
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeError(w, http.StatusMethodNotAllowed, &domain.Error{Kind: domain.KindRefused, Code: CodeMethodNotAllowed, Message: "this route answers " + strings.Join(allowed, ", ")})
}

func (s *Server) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.params.Version})
}

// repos lists the repositories wtm knows, set up, then those not set up a client added, by id.
func (s *Server) repos(w http.ResponseWriter, r *http.Request) {
	repos, err := s.params.Repos(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, domain.AsError(domain.AsErrorParams{Err: err}))
		return
	}

	listed := make([]Repo, 0, len(repos))
	for _, repo := range repos {
		listed = append(listed, Repo{ID: RepoID(repo), Root: repo.Root, CommonDir: repo.CommonDir, SetUp: true})
	}
	s.mu.Lock()
	for _, id := range slices.Sorted(maps.Keys(s.pending)) {
		repo := s.pending[id]
		if slices.ContainsFunc(listed, func(known Repo) bool { return known.ID == id }) {
			delete(s.pending, id) // set up since it was added
			continue
		}
		listed = append(listed, Repo{ID: id, Root: repo.Root, CommonDir: repo.CommonDir})
	}
	s.mu.Unlock()
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
	if !s.track(id, running{repo: RepoID(repo), cancel: cancel}) {
		writeError(w, http.StatusServiceUnavailable, &domain.Error{Kind: domain.KindConflict, Code: CodeStopping, Message: "the server is stopping: run wtm-next ui again"})
		return
	}
	defer s.untrack(id)

	w.Header().Set(HeaderOperation, id)
	options := domain.RunOptions{Surface: domain.SurfaceWeb, CorrelationID: id, DryRun: call.DryRun, ExpectFingerprint: call.ExpectFingerprint}
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

// track counts op among those running, and reports false once the server was asked to stop: under the mutex stopAsked checks, so no operation starts after a stop it let through.
func (s *Server) track(id string, op running) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.asked:
		return false
	default:
	}
	s.running[id] = op
	return true
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

	s.watch(w, r, s.params.Events, call)
}

// watch serves stream to the call as server-sent events: its headers go out once the stream opens or with its first line, so that a refusal before keeps its status; it ends when the client leaves, the server stops or the stream does, a failure after the first line as the event envelope.
func (s *Server) watch(w http.ResponseWriter, r *http.Request, stream api.Stream, call api.Call) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stopped := context.AfterFunc(s.stopping, cancel)
	defer stopped()

	events := newEventStream(w)
	response := stream.Watch(ctx, call, api.Sink{
		Send: func(line json.RawMessage) error {
			events.open()
			return events.raw(line)
		},
		Warn: func(err error) {
			events.open()
			events.named(eventWarning, warningOf(err))
		},
		Open: events.settled,
	})
	switch {
	case response.Envelope.Err == nil:
	case events.isOpen():
		events.named(eventEnvelope, response.Envelope)
	default:
		writeEnvelope(w, response.Envelope)
	}
}

// jobQueries are the queries of a job's lines that answer the stream's fields of the same name.
var jobQueries = []string{"search", "regex", "match_case", "since"}

// jobLines streams the lines of the job {job} of the worktree ?worktree= names, the last ones then each new one until it ends (?follow=false: the last ones alone), or those of its whole log ?search= (?regex=, ?match_case=) or ?since= reads; each line is a line of JSON its stream types.
func (s *Server) jobLines(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repo(w, r)
	if !ok {
		return
	}
	if s.params.Logs.Watch == nil {
		writeError(w, http.StatusNotFound, &domain.Error{Kind: domain.KindInvalid, Code: CodeNotFound, Message: "this server streams no job's lines"})
		return
	}

	query := r.URL.Query()
	call := api.Call{Dir: repo.Root, Answers: []domain.Answered{
		{Path: domain.PathWorktree, Values: []string{query.Get("worktree")}},
		{Path: "job", Values: []string{r.PathValue("job")}},
		{Path: "follow", Values: []string{strconv.FormatBool(query.Get("follow") != "false")}},
	}}
	for _, name := range jobQueries {
		if query.Has(name) {
			call.Answers = append(call.Answers, domain.Answered{Path: name, Values: []string{query.Get(name)}})
		}
	}
	s.watch(w, r, s.params.Logs, call)
}

// A JobLog is a job whose log is asked: the root of its repository, its worktree and its name.
type JobLog struct {
	Dir      string
	Worktree string
	Job      string
}

// jobLog answers the log file of the job {job} of the worktree ?worktree= names, to save: 404 when the job keeps none.
func (s *Server) jobLog(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repo(w, r)
	if !ok {
		return
	}
	if s.params.LogFile == nil {
		writeError(w, http.StatusNotFound, &domain.Error{Kind: domain.KindInvalid, Code: CodeNotFound, Message: "this server serves no job's log"})
		return
	}

	job := r.PathValue("job")
	path, err := s.params.LogFile(r.Context(), JobLog{Dir: repo.Root, Worktree: r.URL.Query().Get("worktree"), Job: job})
	if err != nil {
		writeEnvelope(w, domain.Envelope{Err: domain.AsError(domain.AsErrorParams{Err: err})})
		return
	}
	serveLog(w, r, logParams{Path: path, Name: job})
}

// daemonLog answers the log of wtm's daemon, to save: 404 when it keeps none.
func (s *Server) daemonLog(w http.ResponseWriter, r *http.Request) {
	if s.params.DaemonLog == nil {
		writeError(w, http.StatusNotFound, &domain.Error{Kind: domain.KindInvalid, Code: CodeNotFound, Message: "this server serves no daemon's log"})
		return
	}
	serveLog(w, r, logParams{Path: s.params.DaemonLog(), Name: "daemon"})
}

// logParams is a log to save: the file at Path, saved as Name.log; one at no path, or gone, is said missing.
type logParams struct {
	Path string
	Name string
}

func serveLog(w http.ResponseWriter, r *http.Request, params logParams) {
	file, err := os.Open(params.Path)
	if params.Path == "" || err != nil {
		writeError(w, http.StatusNotFound, &domain.Error{Kind: domain.KindInvalid, Code: CodeLogMissing, Message: params.Name + " keeps no log"})
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, domain.AsError(domain.AsErrorParams{Err: err}))
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": params.Name + ".log"}))
	http.ServeContent(w, r, params.Name+".log", info.ModTime(), file)
}

// A warning is what kept the event stream from a line or a repository for a while, the data of an event named warning.
type warning struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// warningOf is the warning of err, with its code when it has one: daemon.start_failed tells a page wtm's daemon does not answer.
func warningOf(err error) warning {
	failure, ok := errors.AsType[*domain.Error](err)
	if !ok || failure == nil {
		return warning{Message: err.Error()}
	}
	return warning{Message: err.Error(), Code: failure.Code}
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
