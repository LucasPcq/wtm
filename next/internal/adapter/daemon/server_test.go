package daemon_test

import (
	"net"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEveryAnswerIsStampedWithTheProtocolAndTheBuild(t *testing.T) {
	listed := web
	listed.LastLine = &run.Line{At: started, Text: "GET / 200"}
	socket, _ := served(t, &jobs{listed: []run.Job{listed}}, "1.0")

	answers := asked(t, socket, `{"action":"list"}`)

	assert.Equal(t, []map[string]any{{"status": "ok", "protocol": 2.0, "version": "1.0", "jobs": []any{map[string]any{
		"name": "web", "kind": "service", "work_dir": "/wt/a", "state": "running", "hold": "own", "status": "running", "pid": 4242.0,
		"started_at": "2026-10-31T12:00:00Z", "url": "http://localhost:3100", "ports": map[string]any{"PORT": 3100.0},
		"last_line": map[string]any{"at": "2026-10-31T12:00:00.000Z", "text": "GET / 200"},
	}}}}, answers)
}

func TestAStartSendsEachLineThenHowItEnded(t *testing.T) {
	socket, _ := served(t, &jobs{lines: []string{"2 migrations"}, err: &run.Error{Code: run.CodeTaskFailed, Message: "task migrate exited with code 3", ExitCode: new(3)}}, "1.0")

	answers := asked(t, socket, `{"action":"start","spec":{"job":{"name":"migrate","kind":"task","cmd":"make migrate"},"work_dir":"/wt/a"}}`)

	assert.Len(t, answers, 2)
	assert.Equal(t, map[string]any{"status": "line", "protocol": 2.0, "version": "1.0", "at": "2026-10-31T12:00:00.000Z", "text": "2 migrations"}, answers[0])
	assert.Equal(t, "error", answers[1]["status"])
	assert.Equal(t, run.CodeTaskFailed, answers[1]["code"])
	assert.Equal(t, 3.0, answers[1]["exit_code"])
}

func TestAStartIsHandedTheSpecAsTheClientSentIt(t *testing.T) {
	daemonJobs := &jobs{}
	socket, _ := served(t, daemonJobs, "1.0")

	asked(t, socket, `{"action":"start","spec":{"job":{"name":"web","kind":"service","cmd":"pnpm dev","cwd":"apps/web","ports":{"PORT":3000},"url":{"port":"PORT"}},"work_dir":"/wt/a","env":{"PORT":"3100"},"unset":["PORT"],"origin":{"root":"/repo","common_dir":"/repo/.git","branch":"feat/a","correlation_id":"c1"}}}`)

	assert.Equal(t, []any{run.Spec{
		Job:     project.Job{Name: "web", Kind: project.Service, Cmd: "pnpm dev", Cwd: "apps/web", Ports: map[string]int{"PORT": 3000}, URL: &project.URL{Port: "PORT"}},
		WorkDir: "/wt/a", Env: map[string]string{"PORT": "3100"}, Unset: []string{"PORT"},
		Origin: &run.Origin{Root: "/repo", CommonDir: "/repo/.git", Branch: "feat/a", CorrelationID: "c1"},
	}}, daemonJobs.seen())
}

func TestAStopAnswersWhatItDid(t *testing.T) {
	daemonJobs := &jobs{outcome: run.OutcomeNotRunning}
	socket, _ := served(t, daemonJobs, "1.0")

	answers := asked(t, socket, `{"action":"stop","name":"nope","work_dir":"/x","correlation_id":"c2"}`)

	assert.Equal(t, "not_running", answers[0]["outcome"])
	assert.Equal(t, []any{run.Stop{Name: "nope", WorkDir: "/x", CorrelationID: "c2"}}, daemonJobs.seen())
}

func TestARequestTheDaemonCannotReadIsRefusedWithItsCode(t *testing.T) {
	cases := map[string]struct {
		request string
		code    string
	}{
		"not_json":   {request: `notjson`, code: run.CodeInvalidRequest},
		"unknown":    {request: `{"action":"bogus"}`, code: run.CodeUnknownAction},
		"start_none": {request: `{"action":"start"}`, code: run.CodeInvalidRequest},
		"start_anon": {request: `{"action":"start","spec":{}}`, code: run.CodeInvalidRequest},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			socket, _ := served(t, &jobs{}, "1.0")

			answers := asked(t, socket, c.request)

			assert.Equal(t, "error", answers[0]["status"])
			assert.Equal(t, c.code, answers[0]["code"])
		})
	}
}

func TestAShutdownIsAnsweredThenStopsTheDaemon(t *testing.T) {
	socket, shutdowns := served(t, &jobs{}, "1.0")

	answers := asked(t, socket, `{"action":"shutdown"}`)

	assert.Equal(t, "ok", answers[0]["status"])
	assert.Equal(t, int32(1), shutdowns.Load())
}

func TestAClientThatStopsReadingNeverHoldsTheJobsOutput(t *testing.T) {
	daemonJobs := &jobs{lines: slices.Repeat([]string{strings.Repeat("x", 1024)}, 5000), returned: make(chan struct{})}
	socket, _ := served(t, daemonJobs, "1.0")
	conn, err := net.Dial("unix", socket)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte(`{"action":"start","spec":{"job":{"name":"migrate","kind":"task","cmd":"make migrate"},"work_dir":"/wt/a"}}` + "\n"))
	require.NoError(t, err)

	select {
	case <-daemonJobs.returned:
	case <-time.After(5 * time.Second):
		require.Fail(t, "the job's output waited on a client that does not read")
	}
}

func TestEachJobCarriesTheLegacyWordForALegacyClient(t *testing.T) {
	exit := 1
	listed := []run.Job{
		web,
		{Name: "db", WorkDir: "/wt/a", Up: true, State: run.Running, Hold: run.HoldDetached},
		{Name: "cache", WorkDir: "/wt/b", Up: true, Hold: run.HoldClaim},
		{Name: "api", WorkDir: "/wt/a", State: run.Crashed, Hold: run.HoldOwn, ExitCode: &exit},
		{Name: "worker", WorkDir: "/wt/a", State: run.Stopped, Hold: run.HoldOwn, Reaped: true},
		{Name: "migrate", WorkDir: "/wt/a", State: run.Exited, Hold: run.HoldOwn},
	}
	socket, _ := served(t, &jobs{listed: listed}, "1.0")

	answers := asked(t, socket, `{"action":"list"}`)

	var words []any
	for _, job := range answers[0]["jobs"].([]any) {
		words = append(words, job.(map[string]any)["status"])
	}
	assert.Equal(t, []any{"running", "detached", "joined", "crashed", "reaped", "stopped"}, words)
}

func TestAListOfNoJobIsAnEmptyList(t *testing.T) {
	socket, _ := served(t, &jobs{}, "1.0")

	answers := asked(t, socket, `{"action":"list"}`)

	assert.Equal(t, []any{}, answers[0]["jobs"])
}

func TestLinesAreSentThenHowTheJobStands(t *testing.T) {
	daemonJobs := &jobs{lines: []string{"ready", "GET / 200"}}
	socket, _ := served(t, daemonJobs, "1.0")

	answers := asked(t, socket, `{"action":"lines","name":"web","work_dir":"/wt/a","tail":2}`)

	assert.Len(t, answers, 3)
	assert.Equal(t, "ready", answers[0]["text"])
	assert.Equal(t, "GET / 200", answers[1]["text"])
	assert.Equal(t, "ok", answers[2]["status"])
	assert.Equal(t, "web", answers[2]["job"].(map[string]any)["name"])
	assert.Equal(t, []any{run.Tail{Name: "web", WorkDir: "/wt/a", Lines: 2}}, daemonJobs.seen())
}

func TestTheLinesOfAJobTheDaemonDoesNotKnowAreRefused(t *testing.T) {
	socket, _ := served(t, &jobs{err: &run.Error{Code: run.CodeJobNotFound, Message: "no job nope in /wt/a"}}, "1.0")

	answers := asked(t, socket, `{"action":"lines","name":"nope","work_dir":"/wt/a"}`)

	assert.Equal(t, run.CodeJobNotFound, answers[len(answers)-1]["code"])
}

func TestAResizeIsHandedTheSizeAndAnswered(t *testing.T) {
	daemonJobs := &jobs{}
	socket, _ := served(t, daemonJobs, "1.0")

	answers := asked(t, socket, `{"action":"resize","name":"web","work_dir":"/wt/a","cols":120,"rows":40}`)

	assert.Equal(t, "ok", answers[0]["status"])
	assert.Equal(t, []any{run.Window{Name: "web", WorkDir: "/wt/a", Cols: 120, Rows: 40}}, daemonJobs.seen())
}
