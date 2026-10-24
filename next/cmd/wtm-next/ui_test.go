package main

import (
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/webserver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The server runs several operations in one process: the adapters wire.go builds must hold under two calls at once (go test -race).
func TestTwoOperationsAtOnceOnTheServerBothApply(t *testing.T) {
	main := configured(t)
	ownConfigDir(t)
	server := webServing(t)
	repo := registered(t, main)

	var wg sync.WaitGroup
	statuses := make([]int, 3)
	for i, call := range []struct{ path, body string }{
		{"/commands/create", `{"answers":[{"path":"branches","values":["feat/a"]}]}`},
		{"/commands/create", `{"answers":[{"path":"branches","values":["feat/b"]}]}`},
		{"/commands/list", `{}`},
	} {
		wg.Go(func() {
			statuses[i] = post(t, server, "/api/repos/"+repo+call.path, call.body).StatusCode
		})
	}
	wg.Wait()

	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusOK}, statuses)
	assert.DirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-a"))
	assert.DirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-b"))
}

func TestUIReusesTheServerOfItsVersion(t *testing.T) {
	ownConfigDir(t)
	record := webServing(t)

	got, err := ensureServer(t.Context(), time.Hour)

	require.NoError(t, err)
	assert.Equal(t, record, got)
}

func TestUIStartsAServerWhenNoneAnswers(t *testing.T) {
	ownConfigDir(t)
	require.NoError(t, webserver.Files{Dir: webserver.Dir()}.Write(t.Context(), webserver.Record{Port: 1, Version: version, Token: "stale"}))

	_, err := ensureServer(t.Context(), time.Hour)

	assert.ErrorIs(t, err, errServerInTest, "a record no server answers is started over")
}

func TestThePageCarriesTheTokenAndTheRepoInItsFragment(t *testing.T) {
	record := webserver.Record{Port: 4777, Token: "abc"}

	assert.Equal(t, "http://127.0.0.1:4777/#repo=e96269f9d69f&token=abc", pageURL(record, "e96269f9d69f"))
	assert.Equal(t, "http://127.0.0.1:4777/#token=abc", pageURL(record, ""))
}

func TestTheServerServesTheEventStreamOfStreams(t *testing.T) {
	assert.Equal(t, "events", eventsStream().Name)
	assert.Equal(t, version, webParams("t", time.Minute).Version)
}
