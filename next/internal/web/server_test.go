package web

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARequestUnderAPIWithoutTheTokenIsRefused(t *testing.T) {
	server := serving(t, Params{})

	for name, r := range map[string]req{
		"missing": {method: http.MethodGet, path: "/api/version", noToken: true},
		"wrong":   {method: http.MethodGet, path: "/api/version", noToken: true, headers: map[string]string{HeaderToken: "guess"}},
	} {
		t.Run(name, func(t *testing.T) {
			response := do(t, server, r)

			assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
			assert.Equal(t, CodeTokenInvalid, decoded[envelope](t, response).Error.Code)
		})
	}
	assert.Equal(t, http.StatusOK, do(t, server, req{method: http.MethodGet, path: "/api/version"}).StatusCode)
}

func TestThePageIsServedWithoutTheToken(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodGet, path: "/", noToken: true})

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Contains(t, response.Header.Get("Content-Type"), "text/html")
}

func TestAPageOfAnotherOriginIsRefused(t *testing.T) {
	server := serving(t, Params{})

	foreign := do(t, server, req{method: http.MethodGet, path: "/api/version", headers: map[string]string{"Origin": "http://evil.example"}})
	own := do(t, server, req{method: http.MethodGet, path: "/api/version", headers: map[string]string{"Origin": server.URL}})

	assert.Equal(t, http.StatusForbidden, foreign.StatusCode)
	assert.Equal(t, CodeOriginRefused, decoded[envelope](t, foreign).Error.Code)
	assert.Empty(t, foreign.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, http.StatusOK, own.StatusCode)
}

func TestAHostOtherThanTheLoopbackAtTheServersPortIsRefused(t *testing.T) {
	server := serving(t, Params{})
	_, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	require.NoError(t, err)

	for name, host := range map[string]string{
		"rebound":    "evil.example:" + port,
		"other_port": "127.0.0.1:1",
		"no_port":    "localhost",
	} {
		t.Run(name, func(t *testing.T) {
			response := do(t, server, req{method: http.MethodGet, path: "/", headers: map[string]string{"Host": host}})

			assert.Equal(t, http.StatusForbidden, response.StatusCode)
			assert.Equal(t, CodeHostRefused, decoded[envelope](t, response).Error.Code)
		})
	}
	assert.Equal(t, http.StatusOK, do(t, server, req{method: http.MethodGet, path: "/api/version", headers: map[string]string{"Host": "localhost:" + port}}).StatusCode)
}

func TestTheVersionIsTheBinarys(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodGet, path: "/api/version"})

	assert.Equal(t, map[string]string{"version": "1.2.3"}, decoded[map[string]string](t, response))
}

func TestTheReposAreListedByTheirID(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodGet, path: "/api/repos"})

	assert.Equal(t, []Repo{{ID: RepoID(repo), Root: repo.Root, CommonDir: repo.CommonDir}}, decoded[[]Repo](t, response))
}

func TestARepoNoneKnowsIsNotFound(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodGet, path: "/api/repos/abc/commands"})

	assert.Equal(t, http.StatusNotFound, response.StatusCode)
	assert.Equal(t, CodeRepoUnknown, decoded[envelope](t, response).Error.Code)
}

func TestTheCommandsAreTheCatalogues(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodGet, path: repoPath("/commands")})

	listed := decoded[[]map[string]any](t, response)
	require.Len(t, listed, 1)
	assert.Equal(t, "greet", listed[0]["name"])
	assert.NotEmpty(t, listed[0]["form"])
}

func TestTheQuestionsAreAskedOnTheReposRoot(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodPost, path: repoPath("/commands/greet/questions"), body: `{"answers":[{"path":"name","values":["x"]}]}`})

	require.Equal(t, http.StatusOK, response.StatusCode)
	form := decoded[struct {
		Fields []struct {
			Label string `json:"label"`
		} `json:"fields"`
	}](t, response)
	require.Len(t, form.Fields, 1)
	assert.Equal(t, "/code/app:x", form.Fields[0].Label)
}

func TestAnUnknownCommandOrABodyThatIsNoCallIsRefused(t *testing.T) {
	server := serving(t, Params{})

	unknown := do(t, server, req{method: http.MethodPost, path: repoPath("/commands/nope"), body: `{}`})
	garbled := do(t, server, req{method: http.MethodPost, path: repoPath("/commands/greet"), body: `{"answrs":[]}`})

	assert.Equal(t, http.StatusNotFound, unknown.StatusCode)
	assert.Equal(t, CodeCommandUnknown, decoded[envelope](t, unknown).Error.Code)
	assert.Equal(t, http.StatusBadRequest, garbled.StatusCode)
}

func TestARunAnswersItsEnvelopeUnderItsCorrelationID(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodPost, path: repoPath("/commands/greet"), body: `{"answers":[{"path":"name","values":["x"]}]}`})

	require.Equal(t, http.StatusOK, response.StatusCode)
	id := response.Header.Get(HeaderOperation)
	require.NotEmpty(t, id)
	got := decoded[envelope](t, response)
	assert.True(t, got.OK)
	assert.Equal(t, "/code/app:x:web:"+id, got.Op)
}

func TestARefusalHasTheStatusOfItsKind(t *testing.T) {
	for kind, status := range map[domain.Kind]int{
		domain.KindInvalid:      http.StatusBadRequest,
		domain.KindRefused:      http.StatusForbidden,
		domain.KindConflict:     http.StatusConflict,
		domain.KindPrecondition: http.StatusPreconditionFailed,
		domain.KindCancelled:    statusClientClosed,
		domain.KindInternal:     http.StatusInternalServerError,
		"unheard_of":            http.StatusInternalServerError,
	} {
		t.Run(string(kind), func(t *testing.T) {
			refusing := greet(useCase{refusal: &domain.Error{Kind: kind, Code: "greet.refused"}})

			response := do(t, serving(t, Params{Catalogue: api.Catalogue{refusing}}), req{method: http.MethodPost, path: repoPath("/commands/greet"), body: `{"answers":[{"path":"name","values":["x"]}]}`})

			assert.Equal(t, status, response.StatusCode)
			assert.Equal(t, "greet.refused", decoded[envelope](t, response).Error.Code)
		})
	}
}

func TestARunWithEventsStreamsEachAsALineThenItsEnvelope(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodPost, path: repoPath("/commands/greet"), body: `{"answers":[{"path":"name","values":["x"]}],"dry_run":false}`, headers: map[string]string{"Accept": "text/event-stream"}})

	require.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "text/event-stream", response.Header.Get("Content-Type"))
	id := response.Header.Get(HeaderOperation)
	read := events(t, response.Body)
	require.Len(t, read, 2)

	assert.Empty(t, read[0].name)
	line, err := domain.UnmarshalLine([]byte(read[0].data))
	require.NoError(t, err)
	assert.Equal(t, domain.StreamVersion, line.V)
	assert.Equal(t, id, line.CorrelationID)
	assert.Equal(t, &repo, line.Repo)
	assert.Equal(t, domain.UnitStarted{Subject: "x"}, line.Event)

	assert.Equal(t, eventEnvelope, read[1].name)
	var got envelope
	require.NoError(t, json.Unmarshal([]byte(read[1].data), &got))
	assert.True(t, got.OK)
}

// A client that leaves must not cancel the operation it started: only DELETE does.
func TestAClientLeavingCancelsNothing(t *testing.T) {
	started, release, ended := make(chan string, 1), make(chan struct{}), make(chan error, 1)
	server := serving(t, Params{Catalogue: api.Catalogue{greet(useCase{started: started, release: release, ended: ended})}})
	ctx, leave := context.WithCancel(t.Context())
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+repoPath("/commands/greet"), strings.NewReader(`{}`))
	require.NoError(t, err)
	request.Header.Set(HeaderToken, token)
	go func() {
		<-started
		leave()
	}()

	_, err = server.Client().Do(request)
	require.ErrorIs(t, err, context.Canceled)
	// the server notices the closed connection at once; give it the time to (wrongly) cancel
	time.Sleep(200 * time.Millisecond)
	close(release)

	assert.NoError(t, <-ended)
}

func TestDeleteCancelsTheOperationItNames(t *testing.T) {
	started := make(chan string, 1)
	server := serving(t, Params{Catalogue: api.Catalogue{greet(useCase{release: make(chan struct{}), started: started})}})
	done := make(chan *http.Response, 1)
	go func() {
		done <- do(t, server, req{method: http.MethodPost, path: repoPath("/commands/greet"), body: `{"answers":[{"path":"name","values":["x"]}]}`})
	}()
	id := <-started

	cancelled := do(t, server, req{method: http.MethodDelete, path: repoPath("/operations/" + id)})

	assert.Equal(t, http.StatusAccepted, cancelled.StatusCode)
	response := <-done
	assert.Equal(t, statusClientClosed, response.StatusCode)
	assert.Equal(t, "greet.cancelled", decoded[envelope](t, response).Error.Code)
}

func TestAForcedCancelKillsAsASecondInterruptDoes(t *testing.T) {
	started := make(chan string, 1)
	var kills atomic.Int32
	server := serving(t, Params{Catalogue: api.Catalogue{greet(useCase{release: make(chan struct{}), started: started})}, Kill: func() { kills.Add(1) }})
	done := make(chan *http.Response, 1)
	go func() {
		done <- do(t, server, req{method: http.MethodPost, path: repoPath("/commands/greet"), body: `{}`})
	}()
	id := <-started

	cancelled := do(t, server, req{method: http.MethodDelete, path: repoPath("/operations/" + id + "?force=true")})

	assert.Equal(t, http.StatusAccepted, cancelled.StatusCode)
	<-done
	assert.Equal(t, int32(1), kills.Load())
}

func TestCancellingAnOperationNoneRunsIsNotFound(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodDelete, path: repoPath("/operations/nope")})

	assert.Equal(t, http.StatusNotFound, response.StatusCode)
	assert.Equal(t, CodeOperationUnknown, decoded[envelope](t, response).Error.Code)
}

func TestTheEventsOfARepoAreWatchedFromItsRootWithTheirWarnings(t *testing.T) {
	s := &stream{lines: []string{`{"type":"ready"}`}, warnings: []error{errors.New("the bus went away")}, refusal: errors.New("the stream ended")}

	response := do(t, serving(t, Params{Events: s.bind()}), req{method: http.MethodGet, path: "/api/events?repo=" + RepoID(repo)})

	require.Equal(t, http.StatusOK, response.StatusCode)
	read := events(t, response.Body)
	require.Len(t, read, 3)
	assert.Equal(t, sse{name: eventWarning, data: `{"message":"the bus went away"}`}, read[0])
	assert.Equal(t, sse{data: `{"type":"ready"}`}, read[1])
	assert.Equal(t, eventEnvelope, read[2].name, "a failure after the first line ends the stream with its envelope")
	assert.Equal(t, []api.Call{{Dir: repo.Root}}, s.calls)
}

func TestTheEventsOfEveryRepoAreWatchedFromNowhere(t *testing.T) {
	s := &stream{refusal: &domain.Error{Kind: domain.KindPrecondition, Code: "events.config_not_found"}}

	response := do(t, serving(t, Params{Events: s.bind()}), req{method: http.MethodGet, path: "/api/events"})

	assert.Equal(t, http.StatusPreconditionFailed, response.StatusCode, "a refusal before any line keeps its status")
	assert.Equal(t, "events.config_not_found", decoded[envelope](t, response).Error.Code)
	assert.Equal(t, []api.Call{{}}, s.calls)
}

func TestTheSchemaIsNotFoundUntilOneIsPublished(t *testing.T) {
	missing := do(t, serving(t, Params{}), req{method: http.MethodGet, path: "/api/schema"})
	published := do(t, serving(t, Params{Schema: func() []byte { return []byte(`{"$id":"api.v2"}`) }}), req{method: http.MethodGet, path: "/api/schema"})

	assert.Equal(t, http.StatusNotFound, missing.StatusCode)
	assert.Equal(t, CodeSchemaUnavailable, decoded[envelope](t, missing).Error.Code)
	assert.Equal(t, http.StatusOK, published.StatusCode)
	assert.Equal(t, map[string]string{"$id": "api.v2"}, decoded[map[string]string](t, published))
}

func TestAFollowUpIsSpelledAsTheCommandLineRunsIt(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodPost, path: "/api/command-line", body: `{"command":"clean","code":"x","message":"y"}`})

	assert.Equal(t, map[string][]string{"argv": {"wtm", "clean"}}, decoded[map[string][]string](t, response))
}

func TestAnUnknownRouteIsNotFound(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodGet, path: "/api/nope"})

	assert.Equal(t, http.StatusNotFound, response.StatusCode)
	assert.Equal(t, CodeNotFound, decoded[envelope](t, response).Error.Code)
}

func TestServeStopsOnceIdle(t *testing.T) {
	server := New(Params{Idle: 50 * time.Millisecond, Token: token})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- server.Serve(t.Context(), listener) }()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the server did not stop once idle")
	}
}

func TestServeWaitsForTheOperationsRunningWhenItStops(t *testing.T) {
	release := make(chan struct{})
	command := greet(useCase{})
	execute := command.Execute
	command.Execute = func(ctx context.Context, call api.Call) api.Response {
		<-release
		return execute(ctx, call)
	}
	server := New(Params{Catalogue: api.Catalogue{command}, Events: (&stream{}).bind(), Repos: func(context.Context) ([]domain.Repo, error) { return []domain.Repo{repo}, nil }, Token: token, Idle: time.Hour})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, stop := context.WithCancel(t.Context())
	served := make(chan error, 1)
	go func() { served <- server.Serve(ctx, listener) }()

	answered := make(chan int, 1)
	go func() {
		request, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://"+listener.Addr().String()+repoPath("/commands/greet"), strings.NewReader(`{}`))
		request.Header.Set(HeaderToken, token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			answered <- 0
			return
		}
		response.Body.Close()
		answered <- response.StatusCode
	}()
	waitRunning(t, server)
	stop()
	close(release)

	assert.Equal(t, http.StatusOK, <-answered)
	assert.NoError(t, <-served)
}

func TestServeStopsWhenAClientAsksIt(t *testing.T) {
	server := New(Params{Idle: time.Hour, Token: token})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- server.Serve(t.Context(), listener) }()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://"+listener.Addr().String()+"/api/stop", nil)
	require.NoError(t, err)
	request.Header.Set(HeaderToken, token)
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	response.Body.Close()

	assert.Equal(t, http.StatusAccepted, response.StatusCode)
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the server did not stop once asked")
	}
}

// A server that stopped while an operation ran kept its lock until the operation ended, its listener closed: no server answered, and ui said nothing true about why.
func TestStopIsRefusedWhileAnOperationRuns(t *testing.T) {
	started, release := make(chan string, 1), make(chan struct{})
	server := New(Params{Catalogue: api.Catalogue{greet(useCase{started: started, release: release})}, Events: (&stream{}).bind(), Repos: func(context.Context) ([]domain.Repo, error) { return []domain.Repo{repo}, nil }, Token: token, Kill: func() {}, Idle: time.Hour})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	served := make(chan error, 1)
	go func() { served <- server.Serve(t.Context(), listener) }()
	base := "http://" + listener.Addr().String()
	running := make(chan struct{})
	go func() {
		defer close(running)
		request, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, base+repoPath("/commands/greet"), strings.NewReader(`{}`))
		request.Header.Set(HeaderToken, token)
		if response, err := http.DefaultClient.Do(request); err == nil {
			response.Body.Close()
		}
	}()
	id := <-started

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/api/stop", nil)
	require.NoError(t, err)
	request.Header.Set(HeaderToken, token)
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	refused := decoded[struct {
		Operations []string `json:"operations"`
	}](t, response)
	response.Body.Close()

	assert.Equal(t, http.StatusConflict, response.StatusCode)
	assert.Equal(t, []string{id}, refused.Operations)
	close(release)
	<-running
	select {
	case err := <-served:
		t.Fatalf("the server stopped while refusing to: %v", err)
	default:
	}
}

// ConstantTimeCompare of two empty strings is 1: a server given no token let every request through.
func TestAServerWithNoTokenRefusesEveryRequest(t *testing.T) {
	server := httptest.NewServer(New(Params{Idle: time.Hour}))
	t.Cleanup(server.Close)

	response := do(t, server, req{method: http.MethodGet, path: "/api/version", noToken: true})

	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
}

func TestABodyPastItsLimitIsRefused(t *testing.T) {
	big := `{"answers":[{"path":"name","values":["` + strings.Repeat("x", maxBody) + `"]}]}`

	response := do(t, serving(t, Params{}), req{method: http.MethodPost, path: repoPath("/commands/greet"), body: big})

	assert.Equal(t, http.StatusBadRequest, response.StatusCode)
}

func TestServeRefusesAnIdleThatIsNotPositive(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	assert.Error(t, New(Params{Token: token}).Serve(t.Context(), listener))
}

// A stop accepted while a call was still being read let that call start an operation the stopping server then waited for, its lock held and its listener closed.
func TestAnOperationIsRefusedOnceTheServerIsAskedToStop(t *testing.T) {
	server := serving(t, Params{})

	stop := do(t, server, req{method: http.MethodPost, path: "/api/stop"})
	run := do(t, server, req{method: http.MethodPost, path: repoPath("/commands/greet"), body: `{}`})

	assert.Equal(t, http.StatusAccepted, stop.StatusCode)
	assert.Equal(t, http.StatusServiceUnavailable, run.StatusCode)
	assert.Equal(t, CodeStopping, decoded[envelope](t, run).Error.Code)
}

func TestAKnownRouteAskedWithAnotherMethodIsRefusedWithTheMethodsItAnswers(t *testing.T) {
	server := serving(t, Params{})

	wrong := do(t, server, req{method: http.MethodPost, path: "/api/version"})
	unknown := do(t, server, req{method: http.MethodGet, path: "/api/nothing"})

	assert.Equal(t, http.StatusMethodNotAllowed, wrong.StatusCode)
	assert.Equal(t, "GET, HEAD", wrong.Header.Get("Allow"))
	assert.Equal(t, CodeMethodNotAllowed, decoded[envelope](t, wrong).Error.Code)
	assert.Equal(t, http.StatusNotFound, unknown.StatusCode)
	assert.Equal(t, CodeNotFound, decoded[envelope](t, unknown).Error.Code)
}
