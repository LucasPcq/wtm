package main

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheDaemonRunsStopsAndAnnouncesTheJobsItsClientsAsk(t *testing.T) {
	socket := servingDaemon(t)
	client := daemon.Daemon{SocketPath: socket, Version: version}
	lines, err := bus.Bus{SocketPath: socket}.Subscribe(t.Context(), nil)
	require.NoError(t, err)
	dir := t.TempDir()
	origin := &run.Origin{Root: "/repo", CommonDir: "/repo/.git", Branch: "feat/a"}

	var printed []string
	migrate, err := client.Start(t.Context(), run.Spec{Job: project.Job{Name: "migrate", Kind: project.Task, Cmd: "echo migrated"}, WorkDir: dir, Origin: origin}, func(line run.Line) { printed = append(printed, line.Text) })
	require.NoError(t, err)
	web, err := client.Start(t.Context(), run.Spec{Job: project.Job{Name: "web", Kind: project.Service, Cmd: "sleep 30"}, WorkDir: dir, Origin: origin}, nil)
	require.NoError(t, err)
	listed, err := client.Jobs(t.Context())
	require.NoError(t, err)
	outcome, err := client.Stop(t.Context(), run.Stop{Name: "web", WorkDir: dir, CorrelationID: "stop-1"})
	require.NoError(t, err)

	assert.Equal(t, run.Exited, migrate.State)
	assert.Equal(t, []string{"migrated"}, printed)
	assert.True(t, web.Up)
	assert.Positive(t, web.PID)
	assert.Equal(t, []string{"web"}, run.UpIn(listed, dir))
	assert.Equal(t, run.OutcomeStopped, outcome)
	var announced []string
	for range 4 {
		select {
		case payload := <-lines:
			line, err := domain.UnmarshalLine(payload)
			require.NoError(t, err)
			announced = append(announced, domain.EventType(line.Event)+" "+line.CorrelationID)
		case <-time.After(5 * time.Second):
			require.Fail(t, "an event is missing", "announced so far: %v", announced)
		}
	}
	assert.Equal(t, []string{"job.started ", "job.exited ", "job.started ", "job.stopped stop-1"}, announced)
	index, err := os.ReadFile(daemon.IndexPath(socket))
	require.NoError(t, err)
	assert.JSONEq(t, `{"version":2,"jobs":[]}`, string(index))
}

func TestTheDaemonKeepsAJobsLogAndHandsOutItsLinesAndItsTerminal(t *testing.T) {
	socket := servingDaemon(t)
	client := daemon.Daemon{SocketPath: socket, Version: version}
	dir, logs := t.TempDir(), t.TempDir()
	spec := run.Spec{Job: project.Job{Name: "echo", Kind: project.Service, Cmd: "stty -echo; echo ready; exec cat"}, WorkDir: dir, LogDir: logs}
	_, err := client.Start(t.Context(), spec, nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		jobs, err := client.Jobs(t.Context())
		return err == nil && len(jobs) == 1 && jobs[0].LastLine != nil && jobs[0].LastLine.Text == "ready"
	}, 5*time.Second, 20*time.Millisecond)

	followed := make(chan string, 8)
	go func() {
		_, _ = client.Lines(t.Context(), run.Tail{Name: "echo", WorkDir: dir, Follow: true}, func(line run.Line) { followed <- line.Text })
		close(followed)
	}()
	terminal, err := client.Attach(t.Context(), run.Window{Name: "echo", WorkDir: dir, Cols: 80, Rows: 24})
	require.NoError(t, err)
	defer terminal.Close()
	_, err = terminal.Write([]byte("typed\n"))
	require.NoError(t, err)

	assert.Equal(t, "ready", <-followed)
	assert.Equal(t, "typed", <-followed)
	seen := make([]byte, 64)
	n, err := io.ReadAtLeast(terminal, seen, len("ready"))
	require.NoError(t, err)
	assert.Contains(t, string(seen[:n]), "ready")
	_, err = client.Stop(t.Context(), run.Stop{Name: "echo", WorkDir: dir})
	require.NoError(t, err)
	for range followed {
	}
	log, err := os.ReadFile(filepath.Join(logs, "echo.log"))
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^\S+  ready\n\S+  typed\n$`, string(log))
}

func TestTheDaemonsProxyServesAServiceUnderItsNameWhileItRuns(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "web via "+r.Host) }))
	t.Cleanup(backend.Close)
	backendPort := strconv.Itoa(backend.Listener.Addr().(*net.TCPAddr).Port)
	free, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	proxyPort := free.Addr().(*net.TCPAddr).Port
	require.NoError(t, free.Close())
	socket := serving(t, daemonParams{ProxyPort: proxyPort})
	client := daemon.Daemon{SocketPath: socket, Version: version}
	spec := run.Spec{
		Job:     project.Job{Name: "web", Kind: project.Service, Cmd: "sleep 30", Ports: map[string]int{"PORT": 3000}, URL: &project.URL{Port: "PORT"}},
		WorkDir: t.TempDir(),
		Env:     map[string]string{"PORT": backendPort, "WTM_WORKTREE": "feat-a", "WTM_PROJECT": "acme"},
		Routes:  []run.Route{{Job: "web", Host: "web.feat-a.acme.localhost", Port: "PORT"}},
	}

	web, err := client.Start(t.Context(), spec, nil)
	require.NoError(t, err)
	body := proxied(t, proxyPort, "web.feat-a.acme.localhost")
	_, err = client.Stop(t.Context(), run.Stop{Name: "web", WorkDir: spec.WorkDir})
	require.NoError(t, err)

	assert.Equal(t, "http://web.feat-a.acme.localhost:"+strconv.Itoa(proxyPort), web.URL)
	assert.Equal(t, "web via web.feat-a.acme.localhost", body)
	assert.Contains(t, proxied(t, proxyPort, "web.feat-a.acme.localhost"), "no job is published under web.feat-a.acme.localhost", "withdrawn once stopped")
}

func TestWhyTheDaemonServesNoProxyIsInItsLog(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = busy.Close() })
	taken := busy.Addr().(*net.TCPAddr).Port

	unreadable := serving(t, daemonParams{ProxyProblem: errors.New("invalid [proxy] port 99999 in config.toml")})
	moved := serving(t, daemonParams{ProxyPort: taken})

	assert.Equal(t, "run proxy: invalid [proxy] port 99999 in config.toml — jobs keep their own ports\n", daemonLog(t, unreadable), "said once the log was emptied")
	assert.Empty(t, daemonLog(t, moved), "a port taken moves the proxy to the next, silently")
}

func TestARestartLeavesADaemonOfThisBuildAlone(t *testing.T) {
	r := &restarting{}

	err := restartDaemon(t.Context(), r.params(nil, true, false))

	require.NoError(t, err)
	assert.Zero(t, r.shutdowns)
	assert.Contains(t, r.out.String(), "nothing to restart")
}

func TestARestartSaysWhatStopsAndWhatIsKeptThenAsks(t *testing.T) {
	skewed := &run.Skewed{Skew: run.SkewLegacy}
	for name, c := range map[string]struct {
		answer string
		ask    bool
		yes    bool
		kind   domain.Kind
		stops  int
	}{
		"confirmed":  {answer: "y\n", ask: true, stops: 1},
		"declined":   {answer: "n\n", ask: true, kind: domain.KindCancelled},
		"unattended": {kind: domain.KindInvalid},
		"yes":        {yes: true, stops: 1},
	} {
		t.Run(name, func(t *testing.T) {
			r := &restarting{jobs: []run.Job{
				{Name: "web", WorkDir: "/wt/a", Up: true, Hold: run.HoldOwn},
				{Name: "cache", WorkDir: "/wt/a", Up: true, Hold: run.HoldDetached},
				{Name: "db", WorkDir: "/wt/b", Up: true, Hold: run.HoldClaim},
			}, in: c.answer}

			err := restartDaemon(t.Context(), r.params(skewed, c.ask, c.yes))

			assert.Equal(t, c.stops, r.shutdowns)
			assert.Contains(t, r.out.String(), "Stops:  web (/wt/a)")
			assert.Contains(t, r.out.String(), "Kept:   cache (/wt/a), db (/wt/b)")
			if c.kind == "" {
				require.NoError(t, err)
				return
			}
			refused, ok := errors.AsType[*domain.Error](err)
			require.True(t, ok, err)
			assert.Equal(t, c.kind, refused.Kind)
		})
	}
}
