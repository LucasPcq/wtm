package runlogs_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

var (
	migrate = domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate"}
	api     = domain.JobConfig{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev"}
	docker  = domain.JobConfig{Name: "docker", Kind: domain.JobKindService, Cmd: "docker compose up -d", Stop: "docker compose down"}
)

var phaseNames = map[runlogs.Phase]string{
	runlogs.PhaseStarting: "starting",
	runlogs.PhaseOutput:   "output",
	runlogs.PhaseStarted:  "started",
	runlogs.PhaseDone:     "done",
	runlogs.PhaseFailed:   "failed",
	runlogs.PhaseAborted:  "aborted",
	runlogs.PhaseReady:    "ready",
}

func trace(sink *runlogstest.Sink) string {
	steps := make([]string, 0, len(sink.Events))
	for _, event := range sink.Events {
		steps = append(steps, phaseNames[event.Phase]+":"+event.Job)
	}
	return strings.Join(steps, " ")
}

func run(t *testing.T, service *runlogstest.Service, jobs ...domain.JobConfig) (runlogs.Outcome, *runlogstest.Sink) {
	t.Helper()
	sink := &runlogstest.Sink{}
	outcome, err := runlogs.Run(t.Context(), runlogs.RunParams{
		Service: service,
		Sink:    sink,
		Jobs:    jobs,
		WorkDir: "/work/api",
		LogDir:  "/state/logs/api",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return outcome, sink
}

func TestOutcomeRecorded(t *testing.T) {
	cases := []struct {
		label   string
		outcome runlogs.Outcome
		want    bool
	}{
		{label: "nothing was reported", outcome: runlogs.Outcome{}, want: false},
		{label: "the run reached its jobs", outcome: runlogs.Outcome{Steps: 3}, want: true},
		{label: "it ended on one of them", outcome: runlogs.Outcome{Failed: "migrate"}, want: true},
		{label: "a detach kept what it started", outcome: runlogs.Outcome{Started: []string{"api"}}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := tc.outcome.Recorded(); got != tc.want {
				t.Fatalf("Recorded() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRunStartsEveryJobInDeclaredOrder(t *testing.T) {
	service := &runlogstest.Service{}

	outcome, sink := run(t, service, migrate, docker, api)

	if got := service.StartedNames(); !reflect.DeepEqual(got, []string{"migrate", "docker", "api"}) {
		t.Fatalf("started %v, want the declared order", got)
	}
	if got := trace(sink); got != "starting:migrate done:migrate starting:docker started:docker starting:api started:api ready:" {
		t.Fatalf("trace = %q", got)
	}
	if !reflect.DeepEqual(outcome.Completed, []string{"migrate"}) {
		t.Fatalf("completed %v, want the task", outcome.Completed)
	}
	if !reflect.DeepEqual(outcome.Started, []string{"docker", "api"}) {
		t.Fatalf("started %v, want both services", outcome.Started)
	}
	if outcome.Aborted() || outcome.NotStarted != nil {
		t.Fatalf("outcome reports an abort: %+v", outcome)
	}
	if outcome.Steps != 3 {
		t.Fatalf("steps = %d, want 3", outcome.Steps)
	}

	want := []domain.JobActionResult{
		{Name: "migrate", Status: domain.JobActionDone},
		{Name: "docker", Status: domain.JobActionStarted},
		{Name: "api", Status: domain.JobActionStarted},
	}
	if !reflect.DeepEqual(outcome.Results, want) {
		t.Fatalf("results %+v, want %+v", outcome.Results, want)
	}

	ready, found := sink.Last(runlogs.PhaseReady)
	if !found {
		t.Fatal("no ready event")
	}
	if !reflect.DeepEqual(ready.Outcome, outcome) {
		t.Fatalf("ready carries %+v, want the returned outcome", ready.Outcome)
	}
}

func TestRunAbortsOnAFailingTaskAndReportsThePartialState(t *testing.T) {
	service := &runlogstest.Service{
		Refusals: map[string]string{"migrate": "task migrate failed: exit status 1"},
	}

	outcome, sink := run(t, service, docker, migrate, api)

	if got := service.StartedNames(); !reflect.DeepEqual(got, []string{"docker", "migrate"}) {
		t.Fatalf("started %v, want nothing after the failure", got)
	}
	if outcome.Failed != "migrate" || outcome.FailedStep != 2 || outcome.Steps != 3 {
		t.Fatalf("failure placed at %q %d/%d, want migrate 2/3", outcome.Failed, outcome.FailedStep, outcome.Steps)
	}
	if !reflect.DeepEqual(outcome.Started, []string{"docker"}) {
		t.Fatalf("left running %v, want docker", outcome.Started)
	}
	if !reflect.DeepEqual(outcome.NotStarted, []string{"api"}) {
		t.Fatalf("not started %v, want api", outcome.NotStarted)
	}
	if outcome.Completed != nil {
		t.Fatalf("completed %v, want nothing", outcome.Completed)
	}

	failed, found := sink.Last(runlogs.PhaseFailed)
	if !found || failed.Job != "migrate" || failed.Reason != "task migrate failed: exit status 1" {
		t.Fatalf("failed event = %+v", failed)
	}

	aborted, found := sink.Last(runlogs.PhaseAborted)
	if !found {
		t.Fatal("no aborted event")
	}
	if !reflect.DeepEqual(aborted.Outcome, outcome) {
		t.Fatalf("abort reports %+v, want the returned outcome", aborted.Outcome)
	}

	last := outcome.Results[len(outcome.Results)-1]
	if last.Status != domain.JobActionError || last.Message != "task migrate failed: exit status 1" {
		t.Fatalf("last result = %+v, want the failure and its reason", last)
	}
}

// What the failing job printed is the only account of why a run stopped for a
// surface that never showed it live — `run up --output json`, a CI log, an agent
// reading the result — and the daemon's one-line reason is not it.
func TestRunCarriesTheFailedJobsOutputAndExitCode(t *testing.T) {
	service := &runlogstest.Service{
		Output: map[string][]string{
			"docker":  {"docker is up\n"},
			"migrate": {"applying 001\n", "ERROR: relation \"users\" already exists\n"},
		},
		Refusals:  map[string]string{"migrate": "task migrate failed: exit status 1"},
		ExitCodes: map[string]int{"migrate": 1},
	}

	outcome, sink := run(t, service, docker, migrate, api)

	want := "applying 001\nERROR: relation \"users\" already exists\n"
	if got := string(outcome.FailedOutput); got != want {
		t.Fatalf("failed output = %q, want %q", got, want)
	}
	if outcome.FailedExitCode == nil || *outcome.FailedExitCode != 1 {
		t.Fatalf("failed exit code = %v, want 1", outcome.FailedExitCode)
	}

	aborted, found := sink.Last(runlogs.PhaseAborted)
	if !found {
		t.Fatal("no aborted event")
	}
	if string(aborted.Outcome.FailedOutput) != want {
		t.Fatalf("the abort reports %q as the failure's output", aborted.Outcome.FailedOutput)
	}
}

func TestRunCarriesNoOutputForAJobThatNeverPrinted(t *testing.T) {
	service := &runlogstest.Service{
		Output:   map[string][]string{"docker": {"docker is up\n"}},
		Refusals: map[string]string{"api": "port 3000 already in use"},
	}

	outcome, _ := run(t, service, docker, api)

	if outcome.FailedOutput != nil {
		t.Fatalf("failed output = %q, want the previous job's lines left out of it", outcome.FailedOutput)
	}
	if outcome.FailedExitCode != nil {
		t.Fatalf("failed exit code = %v, want none for a job that never ran", *outcome.FailedExitCode)
	}
}

func TestRunTreatsAnAlreadyRunningLauncherAsStarted(t *testing.T) {
	service := &runlogstest.Service{
		Refusals: map[string]string{"docker": "job docker " + domain.JobAlreadyRunningSuffix},
	}

	outcome, sink := run(t, service, docker, api)

	if outcome.Aborted() {
		t.Fatalf("a repeat start aborted the profile: %+v", outcome)
	}
	if !reflect.DeepEqual(outcome.Started, []string{"docker", "api"}) {
		t.Fatalf("started %v, want both", outcome.Started)
	}

	started, found := sink.Last(runlogs.PhaseStarted)
	if !found {
		t.Fatal("no started event")
	}
	if started.Job != "api" || started.AlreadyRunning {
		t.Fatalf("last started event = %+v, want api started for real", started)
	}
	if sink.Events[1].Job != "docker" || !sink.Events[1].AlreadyRunning {
		t.Fatalf("docker event = %+v, want it marked already running", sink.Events[1])
	}
}

// A task is a step to run, not a state to reach: the daemon refusing it because
// it is already running means it has not run here, so the profile stops.
func TestRunAbortsOnAnAlreadyRunningTask(t *testing.T) {
	service := &runlogstest.Service{
		Refusals: map[string]string{"migrate": "job migrate " + domain.JobAlreadyRunningSuffix},
	}

	outcome, _ := run(t, service, migrate, api)

	if outcome.Failed != "migrate" {
		t.Fatalf("outcome = %+v, want the task to abort the profile", outcome)
	}
}

func TestRunAbortsWhenTheDaemonCannotBeReached(t *testing.T) {
	service := &runlogstest.Service{
		Errors: map[string]error{"api": errors.New("connect to daemon: no such file")},
	}

	outcome, sink := run(t, service, docker, api)

	if outcome.Failed != "api" || outcome.FailedStep != 2 {
		t.Fatalf("outcome = %+v, want api to abort at step 2", outcome)
	}
	failed, _ := sink.Last(runlogs.PhaseFailed)
	if failed.Reason != "connect to daemon: no such file" {
		t.Fatalf("reason = %q, want the transport error", failed.Reason)
	}
}

func TestRunForwardsAJobsOutputAsItStarts(t *testing.T) {
	service := &runlogstest.Service{
		Output: map[string][]string{"migrate": {"applying 001\n", "applying 002\n"}},
	}

	_, sink := run(t, service, migrate)

	if got := trace(sink); got != "starting:migrate output:migrate output:migrate done:migrate ready:" {
		t.Fatalf("trace = %q, want the output before the conclusion", got)
	}
	if got := string(sink.Events[1].Chunk); got != "applying 001\n" {
		t.Fatalf("chunk = %q, want the bytes untouched", got)
	}
	if sink.Events[1].Step != 1 || sink.Events[1].Steps != 1 {
		t.Fatalf("output event = %+v, want it placed in the sequence", sink.Events[1])
	}
}

// Detaching ends the reporting, not the run: the daemon keeps what it started,
// the sequence stops where it stands, and nothing more is emitted to a surface
// that is gone.
func TestRunStopsReportingWhenTheSurfaceDetaches(t *testing.T) {
	ctx, detach := context.WithCancel(t.Context())
	sink := &runlogstest.Sink{}
	service := &runlogstest.Service{}
	service.Starting = func(job string) {
		if job == "migrate" {
			detach()
		}
	}

	outcome, err := runlogs.Run(ctx, runlogs.RunParams{
		Service: service,
		Sink:    sink,
		Jobs:    []domain.JobConfig{docker, migrate, api},
		WorkDir: "/work/api",
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run: %v, want context.Canceled", err)
	}
	if got := service.StartedNames(); !reflect.DeepEqual(got, []string{"docker", "migrate"}) {
		t.Fatalf("started %v, want the sequence stopped at the detach", got)
	}
	if outcome.Aborted() {
		t.Fatalf("a detach reads as an abort: %+v", outcome)
	}
	if !reflect.DeepEqual(outcome.Started, []string{"docker"}) {
		t.Fatalf("left running %v, want docker — a detach tears nothing down", outcome.Started)
	}
	// migrate is not among them: the detach landed while its request was in
	// flight, and the daemon took it — StartedNames says so. Naming a job that
	// is running among the ones never started is what made leaving the view look
	// like it had killed the job.
	if !reflect.DeepEqual(outcome.NotStarted, []string{"api"}) {
		t.Fatalf("not started %v, want only the job the detach really cut short", outcome.NotStarted)
	}
	// migrate was announced before the detach reached the runner; nothing it
	// printed, nor how it ended, is reported after.
	if got := trace(sink); got != "starting:docker started:docker starting:migrate" {
		t.Fatalf("trace = %q, want nothing emitted past the detach", got)
	}
}

// The detach can also land between two jobs, with nothing in flight to break.
func TestRunStopsTheSequenceWhenTheDetachLandsBetweenTwoJobs(t *testing.T) {
	ctx, detach := context.WithCancel(t.Context())
	sink := &runlogstest.Sink{}
	service := &runlogstest.Service{}
	service.Answered = func(job string) {
		if job == "docker" {
			detach()
		}
	}

	outcome, err := runlogs.Run(ctx, runlogs.RunParams{
		Service: service,
		Sink:    sink,
		Jobs:    []domain.JobConfig{docker, migrate, api},
		WorkDir: "/work/api",
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run: %v, want context.Canceled", err)
	}
	if got := service.StartedNames(); !reflect.DeepEqual(got, []string{"docker"}) {
		t.Fatalf("started %v, want the sequence stopped at the detach", got)
	}
	if !reflect.DeepEqual(outcome.Started, []string{"docker"}) {
		t.Fatalf("left running %v, want the launcher the daemon did start", outcome.Started)
	}
	if !reflect.DeepEqual(outcome.NotStarted, []string{"migrate", "api"}) {
		t.Fatalf("not started %v, want what the detach cut short", outcome.NotStarted)
	}
	if got := trace(sink); got != "starting:docker" {
		t.Fatalf("trace = %q, want nothing emitted past the detach", got)
	}
}

func TestRunEmitsNothingToASurfaceThatDetachedFirst(t *testing.T) {
	ctx, detach := context.WithCancel(t.Context())
	detach()
	sink := &runlogstest.Sink{}

	outcome, err := runlogs.Run(ctx, runlogs.RunParams{
		Service: &runlogstest.Service{},
		Sink:    sink,
		Jobs:    []domain.JobConfig{docker, api},
		WorkDir: "/work/api",
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run: %v, want context.Canceled", err)
	}
	if len(sink.Events) != 0 {
		t.Fatalf("emitted %+v to a surface that had already gone", sink.Events)
	}
	if !reflect.DeepEqual(outcome.NotStarted, []string{"docker", "api"}) {
		t.Fatalf("not started %v, want every job", outcome.NotStarted)
	}
}

func TestRunWithoutAServiceIsRefused(t *testing.T) {
	_, err := runlogs.Run(t.Context(), runlogs.RunParams{Jobs: []domain.JobConfig{api}})
	if !errors.Is(err, domain.ErrRunServiceRequired) {
		t.Fatalf("Run without a service: %v, want ErrRunServiceRequired", err)
	}
}

func TestRunWithoutASinkStillRuns(t *testing.T) {
	service := &runlogstest.Service{}

	outcome, err := runlogs.Run(t.Context(), runlogs.RunParams{Service: service, Jobs: []domain.JobConfig{api}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !reflect.DeepEqual(outcome.Started, []string{"api"}) {
		t.Fatalf("started %v, want api", outcome.Started)
	}
}

func TestRunEmitsJobURL(t *testing.T) {
	job := domain.JobConfig{
		Name:  "web",
		Kind:  domain.JobKindService,
		Cmd:   "pnpm dev",
		Ports: map[string]int{"PORT": 3000},
		URL:   &domain.JobURLConfig{Port: "PORT"},
	}
	service := &runlogstest.Service{Ports: map[string]map[string]int{"web": {"PORT": 3010}}}
	sink := &runlogstest.Sink{}

	if _, err := runlogs.Run(context.Background(), runlogs.RunParams{
		Service: service,
		Sink:    sink,
		Jobs:    []domain.JobConfig{job},
		WorkDir: "/w",
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, e := range sink.Events {
		if e.Phase == runlogs.PhaseStarted && e.URL != "http://localhost:3010" {
			t.Errorf("started URL = %q, want http://localhost:3010", e.URL)
		}
	}
}

func TestRunEmitsTheNamedURLWhenTheProxyServes(t *testing.T) {
	job := domain.JobConfig{
		Name:  "web",
		Kind:  domain.JobKindService,
		Cmd:   "pnpm dev",
		Ports: map[string]int{"PORT": 3000},
		URL:   &domain.JobURLConfig{Port: "PORT"},
	}
	service := &runlogstest.Service{Ports: map[string]map[string]int{"web": {"PORT": 3010}}, ProxyPort: 4000}
	sink := &runlogstest.Sink{}

	if _, err := runlogs.Run(context.Background(), runlogs.RunParams{
		Service:   service,
		Sink:      sink,
		Jobs:      []domain.JobConfig{job},
		WorkDir:   "/w",
		Env:       map[string]string{domain.EnvWorktree: "feat-auth"},
		Project:   "myapp",
		ProxyPort: 4000,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	const want = "http://web.feat-auth.myapp.localhost:4000"
	for _, e := range sink.Events {
		if e.Phase == runlogs.PhaseStarted && e.URL != want {
			t.Errorf("started URL = %q, want %q", e.URL, want)
		}
	}
	// The daemon is told the same name the event shows, or the browser would
	// resolve a route nothing registered.
	routes := service.Started[0].Routes
	if len(routes) != 1 || routes[0].Host != "web.feat-auth.myapp.localhost" {
		t.Errorf("Routes = %+v, want the host the URL names", routes)
	}
}

func TestRunSendsTheNamesARunnerHoldsForItsChildren(t *testing.T) {
	// The shape the surface hands over: one job to start, carrying the ports of
	// the apps it runs, and the whole declaration beside it.
	runner := domain.JobConfig{
		Name: "dev", Kind: domain.JobKindService, Cmd: "turbo run dev",
		Ports: map[string]int{"PORT": 3000},
		Runs:  []string{"web"},
	}
	web := domain.JobConfig{
		Name: "web", Kind: domain.JobKindService, Cmd: "vite", Cwd: "apps/web",
		Ports: map[string]int{"PORT": 3000},
		URL:   &domain.JobURLConfig{Port: "PORT"},
	}
	service := &runlogstest.Service{Ports: map[string]map[string]int{"dev": {"PORT": 3010}}, ProxyPort: 4000}

	if _, err := runlogs.Run(context.Background(), runlogs.RunParams{
		Service:   service,
		Jobs:      []domain.JobConfig{runner},
		Declared:  []domain.JobConfig{runner, web},
		WorkDir:   "/w",
		Env:       map[string]string{domain.EnvWorktree: "feat-auth"},
		Project:   "myapp",
		ProxyPort: 4000,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	routes := service.Started[0].Routes
	if len(routes) != 1 {
		t.Fatalf("Routes = %+v, want the one name the runner holds", routes)
	}
	if routes[0].Job != "web" || routes[0].Host != "web.feat-auth.myapp.localhost" {
		t.Errorf("route = %+v, want the child published under its own name", routes[0])
	}
}

func TestRunReportsWhereTheAppsARunnerStartedAnswer(t *testing.T) {
	runner := domain.JobConfig{
		Name: "dev", Kind: domain.JobKindService, Cmd: "turbo run dev",
		Ports: map[string]int{"PORT": 3000, "API_PORT": 4000},
		Runs:  []string{"web", "api"},
	}
	web := domain.JobConfig{
		Name: "web", Kind: domain.JobKindService, Cwd: "apps/web",
		Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"},
	}
	api := domain.JobConfig{
		Name: "api", Kind: domain.JobKindService, Cwd: "apps/api",
		Ports: map[string]int{"API_PORT": 4000}, URL: &domain.JobURLConfig{Port: "API_PORT"},
	}
	service := &runlogstest.Service{
		Ports:     map[string]map[string]int{"dev": {"PORT": 3010, "API_PORT": 4010}},
		ProxyPort: 4000,
	}
	sink := &runlogstest.Sink{}

	outcome, err := runlogs.Run(context.Background(), runlogs.RunParams{
		Service:   service,
		Sink:      sink,
		Jobs:      []domain.JobConfig{runner},
		Declared:  []domain.JobConfig{runner, web, api},
		WorkDir:   "/w",
		Env:       map[string]string{domain.EnvWorktree: "feat-auth"},
		Project:   "myapp",
		ProxyPort: 4000,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The runner publishes nothing of its own, so without this the one line the
	// run prints for it carries no address at all.
	held := outcome.Results[0].Held
	if len(held) != 2 {
		t.Fatalf("held = %+v, want one entry per app the runner started", held)
	}
	if held[0].Job != "web" || held[0].URL != "http://web.feat-auth.myapp.localhost:4000" {
		t.Errorf("held[0] = %+v, want web under its own name", held[0])
	}
	// The port is the one the daemon answered it bound, not the declared base.
	if held[1].URL != "http://api.feat-auth.myapp.localhost:4000" {
		t.Errorf("held[1] = %+v, want api under its own name", held[1])
	}
	for _, e := range sink.Events {
		if e.Phase == runlogs.PhaseStarted && len(e.Held) != 2 {
			t.Errorf("started event held = %+v, want the addresses carried to the surface", e.Held)
		}
	}
}

func TestRunHandsOutPortsWhenTheEnvStillSpellsThem(t *testing.T) {
	runner := domain.JobConfig{
		Name: "dev", Kind: domain.JobKindService, Cmd: "turbo run dev",
		Ports: map[string]int{"PORT": 3000}, Runs: []string{"web"},
	}
	web := domain.JobConfig{
		Name: "web", Kind: domain.JobKindService, Cwd: "apps/web",
		Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"},
	}
	service := &runlogstest.Service{Ports: map[string]map[string]int{"dev": {"PORT": 3010}}, ProxyPort: 4000}

	outcome, err := runlogs.Run(context.Background(), runlogs.RunParams{
		Service:       service,
		Jobs:          []domain.JobConfig{runner},
		Declared:      []domain.JobConfig{runner, web},
		WorkDir:       "/w",
		Env:           map[string]string{domain.EnvWorktree: "feat-auth"},
		Project:       "myapp",
		ProxyPort:     4000,
		PortAddressed: true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The name is registered either way, but the .env answers on the port, so
	// the port is the entrance a reader must be given.
	held := outcome.Results[0].Held
	if len(held) != 1 || held[0].URL != "http://localhost:3010" {
		t.Fatalf("held = %+v, want the port the app answers on", held)
	}
}
