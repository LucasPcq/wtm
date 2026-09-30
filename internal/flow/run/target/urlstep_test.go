package target_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
)

func publishing(entries ...domain.JobURLEntry) func(string) []domain.JobURLEntry {
	return func(string) []domain.JobURLEntry { return entries }
}

var (
	apiURL = domain.JobURLEntry{Job: "api", URL: "http://api.wt.repo.localhost"}
	webURL = domain.JobURLEntry{Job: "web", URL: "http://web.wt.repo.localhost"}
)

// A single published address is the answer, not a question — which is what keeps
// `run open` a one-keystroke gesture in the repositories that publish one job.
func TestURLStepIsNotAskedWhenOneJobPublishes(t *testing.T) {
	step := target.URLStep(target.URLParams{Published: publishing(apiURL), Cwd: t.TempDir()})

	skip, reason := step.Skip(flow.Answers{})
	if !skip {
		t.Fatal("the step was asked although a single job publishes a url")
	}
	if reason != domain.RunURLNoChoice {
		t.Errorf("reason = %q, want %q", reason, domain.RunURLNoChoice)
	}
}

func TestURLStepIsAskedWhenSeveralJobsPublish(t *testing.T) {
	step := target.URLStep(target.URLParams{Published: publishing(apiURL, webURL), Cwd: t.TempDir()})

	if skip, _ := step.Skip(flow.Answers{}); skip {
		t.Fatal("the step was skipped although two jobs publish a url")
	}

	content, err := step.Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(content.Options) != 2 {
		t.Fatalf("got %d options, want 2", len(content.Options))
	}
	// The address is what tells two rows apart when both names read alike.
	if got := content.Options[0].Badges; len(got) != 1 || got[0].Text != apiURL.URL {
		t.Errorf("the option does not carry its address: %+v", got)
	}
}

// The refusal names the jobs it could have meant. The generic "pass --job" would
// be true and useless: the caller does not know what to pass it.
func TestURLStepRefusesAnAmbiguityByNamingTheJobs(t *testing.T) {
	step := target.URLStep(target.URLParams{Published: publishing(apiURL, webURL), Cwd: t.TempDir()})

	_, err := step.Resolve(flow.Answers{})
	if !errors.Is(err, domain.ErrJobAmbiguous) {
		t.Fatalf("err = %v, want ErrJobAmbiguous", err)
	}
	if !strings.Contains(err.Error(), "api") || !strings.Contains(err.Error(), "web") {
		t.Errorf("the refusal does not name the published jobs: %v", err)
	}
}

// A job's address follows its worktree's ordinal, so the list is rebuilt from
// the worktree answered one step earlier rather than from where the command was
// launched.
func TestURLStepReadsTheWorktreeAnsweredBefore(t *testing.T) {
	var asked string
	step := target.URLStep(target.URLParams{
		Published: func(workDir string) []domain.JobURLEntry {
			asked = workDir
			return []domain.JobURLEntry{apiURL, webURL}
		},
		Cwd: t.TempDir(),
	})

	if _, err := step.Build(flow.NewAnswers(map[string]string{target.KeyWorktree: "/repo/wt-2"})); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if asked != "/repo/wt-2" {
		t.Errorf("the options were built for %q, want the answered worktree", asked)
	}
}
