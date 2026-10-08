package url_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/url"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

var published = domain.RunConfig{
	Addressing: domain.AddressingPorts,
	Jobs: []domain.JobConfig{
		{Name: "api", Kind: domain.JobKindService, Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}},
		{Name: "web", Kind: domain.JobKindService, Ports: map[string]int{"WEB_PORT": 4000}, URL: &domain.JobURLConfig{Port: "WEB_PORT"}},
		{Name: "worker", Kind: domain.JobKindService},
	},
}

func project(t *testing.T) string {
	t.Helper()
	globaldir.Isolate(t)
	repo, err := filepath.EvalSymlinks(gittest.InitRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func run(t *testing.T, repo string, request url.Request) (url.Outcome, error) {
	t.Helper()
	request.Cwd = repo
	request.Config = published
	return url.Run(t.Context(), url.Params{
		Context: flow.Context{ProjectDir: repo, StateDir: filepath.Join(repo, ".git", "wtm")},
		Request: request,
	})
}

func TestURLListsEveryAddressTheWorktreePublishes(t *testing.T) {
	repo := project(t)

	outcome, err := run(t, repo, url.Request{})

	if err != nil {
		t.Fatalf("url: %v", err)
	}
	if outcome.WorkDir != repo {
		t.Errorf("workdir = %s, want %s", outcome.WorkDir, repo)
	}
	if len(outcome.Entries) != 2 || outcome.Entries[0].Job != "api" || outcome.Entries[1].Job != "web" {
		t.Fatalf("entries = %+v, want api and web, nothing for worker", outcome.Entries)
	}
	if !strings.Contains(outcome.Entries[0].URL, ":3000") {
		t.Errorf("api url = %s, want its port on the main checkout", outcome.Entries[0].URL)
	}
}

// Several addresses read as one line is a question nobody can answer here: the
// refusal names the jobs rather than picking one.
func TestURLReadAsOneLineRefusesSeveralAddresses(t *testing.T) {
	repo := project(t)
	outcome, err := run(t, repo, url.Request{})
	if err != nil {
		t.Fatalf("url: %v", err)
	}

	_, err = outcome.One()

	if err == nil || !strings.Contains(err.Error(), "api") || !strings.Contains(err.Error(), "web") {
		t.Fatalf("err = %v, want both jobs named", err)
	}
}

func TestURLNarrowsToTheJobNamed(t *testing.T) {
	repo := project(t)

	outcome, err := run(t, repo, url.Request{Job: "web"})

	if err != nil {
		t.Fatalf("url: %v", err)
	}
	entry, err := outcome.One()
	if err != nil || entry.Job != "web" || !strings.Contains(entry.URL, ":4000") {
		t.Errorf("one = %+v, %v — want web on 4000", entry, err)
	}
}

func TestURLRefusesAnUndeclaredJob(t *testing.T) {
	repo := project(t)

	_, err := run(t, repo, url.Request{Job: "nope"})

	if !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("err = %v, want ErrJobNotFound", err)
	}
}

// A job run.toml declares but that publishes nothing has no address to give,
// and the refusal says which ones do.
func TestURLRefusesAJobThatPublishesNothing(t *testing.T) {
	repo := project(t)

	_, err := run(t, repo, url.Request{Job: "worker"})

	if err == nil || !strings.Contains(err.Error(), "worker") || !strings.Contains(err.Error(), "api") {
		t.Fatalf("err = %v, want worker refused and the publishing jobs named", err)
	}
}

func TestURLRefusesAWorktreeNamedThatDoesNotExist(t *testing.T) {
	repo := project(t)

	_, err := run(t, repo, url.Request{Worktree: "nowhere"})

	if err == nil || !strings.Contains(err.Error(), "nowhere") {
		t.Fatalf("err = %v, want the worktree named", err)
	}
}
