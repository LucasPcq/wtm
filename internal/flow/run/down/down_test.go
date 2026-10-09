package down_test

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/down"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

var declared = domain.RunConfig{
	Jobs: []domain.JobConfig{
		{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm web"},
		{Name: "docs", Kind: domain.JobKindService, Cmd: "pnpm docs"},
	},
	Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"api", "web"}, Default: true}},
}

type recorder struct {
	*flowtest.Recorder
	downed *down.Outcome
}

func (r *recorder) Downed(outcome down.Outcome) error {
	r.downed = &outcome
	return nil
}

func project(t *testing.T) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(gittest.InitRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func addWorktree(t *testing.T, repo, branch string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), branch)
	gittest.Git(t, repo, "worktree", "add", "-b", branch, dir)
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func run(t *testing.T, repo string, request down.Request) (down.Outcome, *recorder, error) {
	t.Helper()
	presenter := &recorder{Recorder: &flowtest.Recorder{}}
	request.Cwd = repo
	request.Config = declared
	outcome, err := down.Run(t.Context(), down.Params{
		Context:   flow.Context{ProjectDir: repo},
		Request:   request,
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	return outcome, presenter, err
}

func statuses(results []domain.JobActionResult) string {
	parts := make([]string, 0, len(results))
	for _, result := range results {
		parts = append(parts, result.Name+"="+result.Status)
	}
	return strings.Join(parts, " ")
}

func TestDownStopsEverythingTheWorktreeHasUp(t *testing.T) {
	repo := project(t)
	daemon := processtest.Serve(t, []domain.JobInfo{
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: repo},
		{Name: "docs", Status: domain.JobStatusRunning, WorkDir: repo},
	})

	outcome, presenter, err := run(t, repo, down.Request{})

	if err != nil {
		t.Fatalf("down: %v", err)
	}
	if len(outcome.Results) != 1 || outcome.Results[0].Branch != "main" || outcome.Results[0].Path != repo {
		t.Fatalf("results = %+v, want main alone", outcome.Results)
	}
	if got := statuses(outcome.Results[0].Jobs); got != "api=stopped docs=stopped" {
		t.Errorf("jobs = %s, want both stopped", got)
	}
	if want := "stop_all:@" + repo; strings.Join(daemon.Actions(), " ") != want {
		t.Errorf("requests = %v, want %s", daemon.Actions(), want)
	}
	if presenter.downed == nil {
		t.Error("the outcome never reached the presenter")
	}
}

// A profile's jobs are stopped one by one, and one that was not up is said to
// be not_running without asking the daemon to stop it.
func TestDownProfileStopsItsJobsAndNamesTheOnesNotUp(t *testing.T) {
	repo := project(t)
	daemon := processtest.Serve(t, []domain.JobInfo{
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: repo},
		{Name: "docs", Status: domain.JobStatusRunning, WorkDir: repo},
	})

	outcome, _, err := run(t, repo, down.Request{Profile: "dev"})

	if err != nil {
		t.Fatalf("down: %v", err)
	}
	if got := statuses(outcome.Results[0].Jobs); got != "api=stopped web=not_running" {
		t.Errorf("jobs = %s, want api stopped and web not_running", got)
	}
	if want := "stop:api@" + repo; strings.Join(daemon.Actions(), " ") != want {
		t.Errorf("requests = %v, want only api stopped, docs left alone", daemon.Actions())
	}
	if got := outcome.Stopped(); len(got) != 1 || got[0].Name != "api" {
		t.Errorf("Stopped() = %+v, want api alone", got)
	}
}

// A job the daemon would not stop is named in the document, and the outcome
// says the run failed.
func TestDownProfileNamesAJobLeftStanding(t *testing.T) {
	repo := project(t)
	daemon := processtest.Serve(t, []domain.JobInfo{{Name: "api", Status: domain.JobStatusRunning, WorkDir: repo}})
	daemon.StopError = "job api refused to stop"

	outcome, _, err := run(t, repo, down.Request{Profile: "dev"})

	if err != nil {
		t.Fatalf("down: %v", err)
	}
	if !outcome.Failed() {
		t.Error("Failed() = false with a job left standing")
	}
	if got := outcome.Results[0].Jobs[0]; got.Status != domain.JobActionError || got.Message != "job api refused to stop" {
		t.Errorf("api = %+v, want the refusal", got)
	}
}

// With no daemon and nothing indexed there is nothing to stop: the document
// keeps its shape, every profile job not_running, and no daemon is started.
func TestDownWithNoDaemonKeepsTheDocumentsShape(t *testing.T) {
	globaldir.Isolate(t)
	repo := project(t)

	outcome, _, err := run(t, repo, down.Request{Profile: "dev"})

	if err != nil {
		t.Fatalf("down: %v", err)
	}
	if !outcome.NoDaemon {
		t.Error("NoDaemon = false")
	}
	if len(outcome.Results) != 1 || outcome.Results[0].Branch != "main" {
		t.Fatalf("results = %+v, want main", outcome.Results)
	}
	if got := statuses(outcome.Results[0].Jobs); got != "api=not_running web=not_running" {
		t.Errorf("jobs = %s, want the profile not_running", got)
	}
}

func TestDownRefusesAnUndeclaredProfile(t *testing.T) {
	repo := project(t)
	daemon := processtest.Serve(t, nil)

	_, _, err := run(t, repo, down.Request{Profile: "nope"})

	if !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("err = %v, want ErrProfileNotFound", err)
	}
	if len(daemon.Actions()) != 0 {
		t.Errorf("requests = %v, want none", daemon.Actions())
	}
}

// --all reaches every worktree of this project holding jobs, and nothing of
// another repository the machine-wide daemon also runs.
func TestDownAllEmptiesThisProjectOnly(t *testing.T) {
	repo := project(t)
	feature := addWorktree(t, repo, "feature")
	elsewhere := project(t)
	daemon := processtest.Serve(t, []domain.JobInfo{
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: repo},
		{Name: "web", Status: domain.JobStatusRunning, WorkDir: feature},
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: elsewhere},
	})

	outcome, _, err := run(t, repo, down.Request{All: true})

	if err != nil {
		t.Fatalf("down --all: %v", err)
	}
	branches := make([]string, 0, len(outcome.Results))
	for _, result := range outcome.Results {
		branches = append(branches, result.Branch)
	}
	if got := strings.Join(branches, ","); got != "main,feature" {
		t.Errorf("worktrees = %s, want main,feature", got)
	}
	for _, action := range daemon.Actions() {
		if strings.HasSuffix(action, "@"+elsewhere) {
			t.Errorf("requests = %v: --all reached another repository", daemon.Actions())
		}
	}
}

func TestDownBackedOutOfSaysAborted(t *testing.T) {
	repo := project(t)
	processtest.Serve(t, nil)
	presenter := &recorder{Recorder: &flowtest.Recorder{}}

	outcome, err := down.Run(t.Context(), down.Params{
		Context:   flow.Context{ProjectDir: repo},
		Request:   down.Request{Cwd: repo, Config: declared},
		Prompter:  &flowtest.ScriptedPrompter{Abort: true},
		Presenter: presenter,
	})

	if err != nil || !outcome.Aborted {
		t.Fatalf("outcome = %+v, err = %v, want an abort", outcome, err)
	}
	if presenter.downed != nil {
		t.Error("an abort reached the presenter as a conclusion")
	}
}

// LUC-276 (a): a crashed job is settled by every shape of `run down`, and never
// reported as stopped: it was not running.
func TestDownSettlesACrashedJobWithoutReportingIt(t *testing.T) {
	repo := project(t)
	daemon := processtest.Serve(t, []domain.JobInfo{
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: repo},
		{Name: "web", Status: domain.JobStatusCrashed, WorkDir: repo},
	})

	outcome, _, err := run(t, repo, down.Request{})

	if err != nil {
		t.Fatalf("down: %v", err)
	}
	if got := statuses(outcome.Results[0].Jobs); got != "api=stopped" {
		t.Errorf("jobs = %s, want api alone", got)
	}
	if want := "stop_all:@" + repo; strings.Join(daemon.Actions(), " ") != want {
		t.Errorf("requests = %v, want %s", daemon.Actions(), want)
	}
}

func TestDownProfileSettlesItsCrashedJob(t *testing.T) {
	repo := project(t)
	daemon := processtest.Serve(t, []domain.JobInfo{
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: repo},
		{Name: "web", Status: domain.JobStatusCrashed, WorkDir: repo},
	})

	outcome, _, err := run(t, repo, down.Request{Profile: "dev"})

	if err != nil {
		t.Fatalf("down: %v", err)
	}
	if got := statuses(outcome.Results[0].Jobs); got != "api=stopped web=not_running" {
		t.Errorf("jobs = %s, want api stopped and web not_running", got)
	}
	if !slices.Contains(daemon.Actions(), "stop:web@"+repo) {
		t.Errorf("requests = %v, want the crashed web settled", daemon.Actions())
	}
}

func TestDownAllSettlesAWorktreeWhoseJobsAllCrashed(t *testing.T) {
	repo := project(t)
	feature := addWorktree(t, repo, "feature")
	daemon := processtest.Serve(t, []domain.JobInfo{
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: repo},
		{Name: "web", Status: domain.JobStatusCrashed, WorkDir: feature},
	})

	outcome, _, err := run(t, repo, down.Request{All: true})

	if err != nil {
		t.Fatalf("down --all: %v", err)
	}
	if len(outcome.Results) != 1 || outcome.Results[0].Branch != "main" {
		t.Errorf("results = %+v, want main alone: feature stopped nothing", outcome.Results)
	}
	if !slices.Contains(daemon.Actions(), "stop_all:@"+feature) {
		t.Errorf("requests = %v, want feature's crash settled", daemon.Actions())
	}
}
