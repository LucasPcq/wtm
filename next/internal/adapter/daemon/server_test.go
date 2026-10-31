package daemon_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
)

func TestEveryAnswerIsStampedWithTheProtocolAndTheBuild(t *testing.T) {
	socket, _ := served(t, &jobs{listed: []run.Job{web}}, "1.0")

	answers := asked(t, socket, `{"action":"list"}`)

	assert.Equal(t, []map[string]any{{"status": "ok", "protocol": 2.0, "version": "1.0", "jobs": []any{map[string]any{
		"name": "web", "kind": "service", "work_dir": "/wt/a", "state": "running", "hold": "own", "pid": 4242.0,
		"started_at": "2026-10-31T12:00:00Z", "url": "http://localhost:3100", "ports": map[string]any{"PORT": 3100.0},
	}}}}, answers)
}

func TestAStartSendsEachLineThenHowItEnded(t *testing.T) {
	socket, _ := served(t, &jobs{lines: []string{"2 migrations"}, err: &run.Error{Code: run.CodeTaskFailed, Message: "task migrate exited with code 3", ExitCode: new(3)}}, "1.0")

	answers := asked(t, socket, `{"action":"start","spec":{"job":{"name":"migrate","kind":"task","cmd":"make migrate"},"work_dir":"/wt/a"}}`)

	assert.Len(t, answers, 2)
	assert.Equal(t, map[string]any{"status": "line", "protocol": 2.0, "version": "1.0", "at": "2026-10-31T12:00:00Z", "text": "2 migrations"}, answers[0])
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
