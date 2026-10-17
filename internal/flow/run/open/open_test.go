package open_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/open"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

var api = domain.JobConfig{Name: "api", Kind: domain.JobKindService, Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}}
var web = domain.JobConfig{Name: "web", Kind: domain.JobKindService, Ports: map[string]int{"WEB_PORT": 4000}, URL: &domain.JobURLConfig{Port: "WEB_PORT"}}

type fixture struct {
	repo   string
	opened []string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	globaldir.Isolate(t)
	repo, err := filepath.EvalSymlinks(gittest.InitRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{repo: repo}
}

type runParams struct {
	Request  open.Request
	Jobs     []domain.JobConfig
	Prompter flow.Prompter
	Open     func(string) error
}

func (f *fixture) run(t *testing.T, params runParams) (open.Outcome, *flowtest.Recorder, error) {
	t.Helper()
	params.Request.Cwd = f.repo
	params.Request.Config = domain.RunConfig{Addressing: domain.AddressingPorts, Jobs: params.Jobs}
	if params.Prompter == nil {
		params.Prompter = flow.Unattended{}
	}
	if params.Open == nil {
		params.Open = func(url string) error {
			f.opened = append(f.opened, url)
			return nil
		}
	}
	presenter := &flowtest.Recorder{}
	outcome, err := open.Run(t.Context(), open.Params{
		Context:   flow.Context{ProjectDir: f.repo, StateDir: filepath.Join(f.repo, ".git", "wtm")},
		Request:   params.Request,
		Prompter:  params.Prompter,
		Presenter: presenter,
		Open:      params.Open,
	})
	return outcome, presenter, err
}

// One address is the answer, not a question: it is opened without asking.
func TestOpenOpensTheOnlyPublishedAddress(t *testing.T) {
	f := newFixture(t)
	prompter := &flowtest.ScriptedPrompter{}

	outcome, _, err := f.run(t, runParams{Jobs: []domain.JobConfig{api}, Prompter: prompter})

	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(f.opened) != 1 || !strings.Contains(f.opened[0], ":3000") || outcome.Entry.Job != "api" {
		t.Errorf("opened = %v, outcome = %+v, want api's address", f.opened, outcome)
	}
	if prompter.AskedKeys() != "" {
		t.Errorf("asked %s, want nothing", prompter.AskedKeys())
	}
}

func TestOpenAsksWhichAddressWhenSeveralArePublished(t *testing.T) {
	f := newFixture(t)
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{target.KeyJob: "web"}}

	if _, _, err := f.run(t, runParams{Jobs: []domain.JobConfig{api, web}, Prompter: prompter}); err != nil {
		t.Fatalf("open: %v", err)
	}
	if prompter.AskedKeys() != target.KeyJob {
		t.Errorf("asked %q, want the job", prompter.AskedKeys())
	}
	if len(f.opened) != 1 || !strings.Contains(f.opened[0], ":4000") {
		t.Errorf("opened = %v, want web's address", f.opened)
	}
}

// Nobody to ask: the refusal names the jobs it could have meant.
func TestAnUnattendedOpenRefusesSeveralAddressesNamingThem(t *testing.T) {
	f := newFixture(t)

	_, _, err := f.run(t, runParams{Jobs: []domain.JobConfig{api, web}})

	if !errors.Is(err, domain.ErrJobAmbiguous) || !strings.Contains(err.Error(), "web") {
		t.Fatalf("err = %v, want the ambiguity with the jobs named", err)
	}
	if len(f.opened) != 0 {
		t.Errorf("opened %v on a refused run", f.opened)
	}
}

func TestOpenOfTheJobNamedAsksNothing(t *testing.T) {
	f := newFixture(t)

	outcome, _, err := f.run(t, runParams{Request: open.Request{Job: "web"}, Jobs: []domain.JobConfig{api, web}})

	if err != nil || outcome.Entry.Job != "web" {
		t.Fatalf("outcome = %+v, err = %v, want web", outcome, err)
	}
}

func TestOpenRefusesAnUndeclaredJobBeforeAnything(t *testing.T) {
	f := newFixture(t)

	_, _, err := f.run(t, runParams{Request: open.Request{Job: "nope"}, Jobs: []domain.JobConfig{api}})

	if !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("err = %v, want ErrJobNotFound", err)
	}
}

func TestOpenWithNothingPublishedSaysSo(t *testing.T) {
	f := newFixture(t)

	_, _, err := f.run(t, runParams{Jobs: []domain.JobConfig{{Name: "worker", Kind: domain.JobKindService}}})

	if !errors.Is(err, domain.ErrJobNonePublished) {
		t.Fatalf("err = %v, want ErrJobNonePublished", err)
	}
}

// The surface's opener failing is the run failing: nothing was opened.
func TestOpenReturnsTheOpenersFailure(t *testing.T) {
	f := newFixture(t)
	cause := errors.New("no browser")

	_, _, err := f.run(t, runParams{Jobs: []domain.JobConfig{api}, Open: func(string) error { return cause }})

	if !errors.Is(err, cause) {
		t.Fatalf("err = %v, want the opener's failure", err)
	}
}

// Under ports addressing no name is published, so there is nothing for the
// .env to disagree with and no warning to give.
func TestOpenUnderPortsAddressingWarnsOfNothing(t *testing.T) {
	f := newFixture(t)

	_, presenter, err := f.run(t, runParams{Jobs: []domain.JobConfig{api}})

	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(presenter.Statuses) != 0 {
		t.Errorf("statuses = %+v, want none", presenter.Statuses)
	}
}

func TestOpenBackedOutOfSaysAborted(t *testing.T) {
	f := newFixture(t)

	outcome, presenter, err := f.run(t, runParams{Jobs: []domain.JobConfig{api, web}, Prompter: &flowtest.ScriptedPrompter{Abort: true}})

	if err != nil || !outcome.Aborted {
		t.Fatalf("outcome = %+v, err = %v, want an abort", outcome, err)
	}
	if len(presenter.Notices) != 1 || len(f.opened) != 0 {
		t.Errorf("notices = %+v, opened = %v, want Aborted and nothing opened", presenter.Notices, f.opened)
	}
}
