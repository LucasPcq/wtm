package web

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/require"
)

// serving starts a server of params over the defaults: greet in the catalogue, an event stream that only waits, repo the one repository known, the token, an hour of idle.
func serving(t *testing.T, params Params) *httptest.Server {
	t.Helper()
	if params.Catalogue == nil {
		params.Catalogue = api.Catalogue{greet(useCase{})}
	}
	if params.Events.Watch == nil {
		params.Events = (&stream{}).bind()
	}
	if params.Repos == nil {
		params.Repos = func(context.Context) ([]domain.Repo, error) { return []domain.Repo{repo}, nil }
	}
	if params.Kill == nil {
		params.Kill = func() {}
	}
	if params.CommandLine == nil {
		params.CommandLine = func(followUp domain.FollowUp) []string { return []string{"wtm", followUp.Command} }
	}
	params.Token = token
	params.Version = "1.2.3"
	params.Idle = time.Hour

	server := httptest.NewServer(New(params))
	t.Cleanup(server.Close)
	return server
}

// A req is one request to a server: its method and path, its body, and its headers besides the token, which it carries unless noToken.
type req struct {
	method  string
	path    string
	body    string
	headers map[string]string
	noToken bool
}

func do(t *testing.T, server *httptest.Server, r req) *http.Response {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), r.method, server.URL+r.path, strings.NewReader(r.body))
	require.NoError(t, err)
	if !r.noToken {
		request.Header.Set(HeaderToken, token)
	}
	for name, value := range r.headers {
		request.Header.Set(name, value)
	}
	if host, ok := r.headers["Host"]; ok {
		request.Host = host
	}

	response, err := server.Client().Do(request)
	require.NoError(t, err)
	t.Cleanup(func() { response.Body.Close() })
	return response
}

// decoded is the JSON body of response.
func decoded[T any](t *testing.T, response *http.Response) T {
	t.Helper()
	var v T
	require.NoError(t, json.NewDecoder(response.Body).Decode(&v))
	return v
}

// envelope is what a test reads of the JSON envelope a response carries.
type envelope struct {
	OK    bool   `json:"ok"`
	Op    string `json:"op"`
	Error *struct {
		Kind   string        `json:"kind"`
		Code   string        `json:"code"`
		Params domain.Params `json:"params"`
	} `json:"error"`
}

// An sse is one server-sent event: its name, empty for a line, and its data.
type sse struct {
	name string
	data string
}

// events reads the server-sent events of body until it ends.
func events(t *testing.T, body io.Reader) []sse {
	t.Helper()
	var read []sse
	var current sse
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			read = append(read, current)
			current = sse{}
		case strings.HasPrefix(line, "event: "):
			current.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			current.data = strings.TrimPrefix(line, "data: ")
		}
	}
	return read
}

func repoPath(suffix string) string {
	return "/api/repos/" + RepoID(repo) + suffix
}

// waitRunning waits until server runs an operation.
func waitRunning(t *testing.T, server *Server) {
	t.Helper()
	require.Eventually(t, func() bool {
		server.mu.Lock()
		defer server.mu.Unlock()
		return len(server.running) > 0
	}, 5*time.Second, 5*time.Millisecond)
}
