package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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
	server, _ := webServing(t, version)
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
	record, _ := webServing(t, version)

	got, err := ensureServer(t.Context(), time.Hour)

	require.NoError(t, err)
	assert.Equal(t, record, got)
}

func TestUIAsksAServerOfAnotherVersionToStopThenStartsItsOwn(t *testing.T) {
	ownConfigDir(t)
	_, stopped := webServing(t, "0.0.0")

	_, err := ensureServer(t.Context(), time.Hour)

	assert.ErrorIs(t, err, errServerInTest)
	select {
	case <-stopped:
	default:
		t.Fatal("the server of another version still runs")
	}
}

func TestUIStartsAServerWhenNoneAnswers(t *testing.T) {
	ownConfigDir(t)
	require.NoError(t, webserver.Files{Dir: webserver.Dir()}.Write(t.Context(), webserver.Record{PID: os.Getpid(), Port: 1, Version: version, Token: "stale"}))

	_, err := ensureServer(t.Context(), time.Hour)

	assert.ErrorIs(t, err, errServerInTest, "a record no server answers is started over")
}

// A record outlives its server: ui signalled the pid it read from it, kill(-1) ending every process of the user, and gave its token to whatever listened on its port.
func TestUITrustsTheRecordOnlyWhileItsServerHoldsTheLock(t *testing.T) {
	ownConfigDir(t)
	var asked atomic.Bool
	impostor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		asked.Store(true)
		_, _ = w.Write([]byte(`{"version":"0.0.0"}`))
	}))
	t.Cleanup(impostor.Close)
	port := impostor.Listener.Addr().(*net.TCPAddr).Port
	// the test's own pid, never another's: a regression signals this test alone
	require.NoError(t, webserver.Files{Dir: webserver.Dir()}.Write(t.Context(), webserver.Record{PID: os.Getpid(), Port: port, Version: "0.0.0", Token: "stale"}))

	_, err := ensureServer(t.Context(), time.Hour)

	assert.ErrorIs(t, err, errServerInTest, "no server holds the lock: one is started")
	assert.False(t, asked.Load(), "the stale record's port was asked, and given its token")
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

// ui asked a server finishing an operation to stop, waited 10 s for a lock it kept, and said it did not stop; a second ui then started a server that left at once.
func TestUILeavesAServerFinishingAnOperationAndSaysSo(t *testing.T) {
	ownConfigDir(t)
	files := webserver.Files{Dir: webserver.Dir()}
	release, err := files.Lock()
	require.NoError(t, err)
	t.Cleanup(release)
	busy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/stop" {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"operations":["op-7"]}`))
			return
		}
		_, _ = w.Write([]byte(`{"version":"0.0.0"}`))
	}))
	t.Cleanup(busy.Close)
	require.NoError(t, files.Write(t.Context(), webserver.Record{PID: os.Getpid(), Port: busy.Listener.Addr().(*net.TCPAddr).Port, Version: "0.0.0", Token: "old"}))

	_, err = ensureServer(t.Context(), time.Hour)

	require.Error(t, err)
	assert.NotErrorIs(t, err, errServerInTest, "no server is started while the old one holds the lock")
	assert.Contains(t, err.Error(), "the web server of wtm 0.0.0 is finishing operation op-7")
}

func TestAnIdleThatIsNotPositiveIsRefused(t *testing.T) {
	assert.Error(t, positive(0))
	assert.Error(t, positive(-time.Second))
	assert.NoError(t, positive(time.Second))
}
