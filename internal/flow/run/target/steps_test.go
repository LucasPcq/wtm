package target_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestWorktreeStepIsNotAskedInASingleWorktreeRepository(t *testing.T) {
	repo := gittest.InitRepo(t)

	step := target.WorktreeStep(target.WorktreeParams{ProjectDir: repo, Current: repo})

	skip, reason := step.Skip(flow.Answers{})
	if !skip {
		t.Fatal("the step was asked in a repository holding one worktree")
	}
	if reason != domain.RunWorktreeOnlyOne {
		t.Errorf("reason = %q, want %q", reason, domain.RunWorktreeOnlyOne)
	}
}

// The current worktree is the answer whenever nobody is asked: standing in a
// worktree is what designates it, which is why `run` needs no picker to have a
// non-interactive path.
func TestWorktreeStepResolvesToTheCurrentWorktree(t *testing.T) {
	repo := gittest.InitRepo(t)

	step := target.WorktreeStep(target.WorktreeParams{ProjectDir: repo, Current: repo})

	answer, err := step.Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// Spelled the way git spells it, whatever the caller passed: the answer is
	// the daemon's key for every job the run touches.
	if answer.Value != target.Root(repo) {
		t.Errorf("Resolve = %q, want the current worktree %q", answer.Value, target.Root(repo))
	}
}

func TestWorktreeStepOffersEveryWorktreeWithItsRunningJobs(t *testing.T) {
	repo, second := repoWithSecondWorktree(t)

	step := target.WorktreeStep(target.WorktreeParams{
		ProjectDir: repo,
		Current:    repo,
		Running:    map[string]int{second: 2},
	})

	if skip, _ := step.Skip(flow.Answers{}); skip {
		t.Fatal("the step was skipped although the repository holds two worktrees")
	}

	content, err := step.Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if content.Start != target.Root(repo) {
		t.Errorf("Start = %q, want the current worktree %q", content.Start, target.Root(repo))
	}
	if len(content.Options) != 2 {
		t.Fatalf("options = %d, want 2", len(content.Options))
	}

	badges := badgesByValue(content.Options)
	if got := badges[second]; got != "2 running" {
		t.Errorf("second worktree badges = %q, want its running count", got)
	}
	if got := badges[target.Root(repo)]; got != domain.RunWorktreeCurrent {
		t.Errorf("current worktree badges = %q, want %q", got, domain.RunWorktreeCurrent)
	}
}

// The answer is a path, because that is the daemon's key; the recap has to read
// back as the branch the user typed.
func TestWorktreeStepSummarizesAPathAsItsBranch(t *testing.T) {
	repo, second := repoWithSecondWorktree(t)

	step := target.WorktreeStep(target.WorktreeParams{ProjectDir: repo, Current: repo})

	if got := step.Summarize(flow.Answer{Value: second}); got != "feature" {
		t.Errorf("Summarize = %q, want the branch name", got)
	}
}

func TestJobStepRefusesInsteadOfPickingWhenNobodyCanBeAsked(t *testing.T) {
	step := target.JobStep(target.JobParams{
		Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}},
		Flag: domain.FlagJob,
	})

	if step.Resolve != nil {
		t.Error("the job step resolves, so an unattended run would answer it silently")
	}
	if step.Flag != domain.FlagJob {
		t.Errorf("Flag = %q, want the step to name --%s when it refuses", step.Flag, domain.FlagJob)
	}
}

func TestJobStepOffersEachJobWithItsKind(t *testing.T) {
	step := target.JobStep(target.JobParams{Jobs: []domain.JobConfig{
		{Name: "web", Kind: domain.JobKindService},
		{Name: "migrate", Kind: domain.JobKindTask},
	}})

	content, err := step.Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	badges := badgesByValue(content.Options)
	if badges["web"] != string(domain.JobKindService) || badges["migrate"] != string(domain.JobKindTask) {
		t.Errorf("badges = %v, want each job tagged with its kind", badges)
	}
}

func TestJobStepSaysSoWhenThereIsNothingToPick(t *testing.T) {
	step := target.JobStep(target.JobParams{})

	if _, err := step.Build(flow.Answers{}); !errors.Is(err, domain.ErrNoJobsDeclared) {
		t.Errorf("Build error = %v, want ErrNoJobsDeclared", err)
	}
}

// One profile — or none — is a question with a single answer, so it is never
// put to anyone.
func TestProfileStepIsNotAskedWithoutAChoice(t *testing.T) {
	step := target.ProfileStep(target.ProfileParams{
		Profiles: []domain.ProfileConfig{{Name: "default"}},
	})

	skip, reason := step.Skip(flow.Answers{})
	if !skip || reason != domain.RunProfileNoChoice {
		t.Errorf("Skip = (%v, %q), want the step skipped", skip, reason)
	}
}

func TestProfileStepPicksExactlyOneProfile(t *testing.T) {
	step := target.ProfileStep(target.ProfileParams{
		Profiles: []domain.ProfileConfig{{Name: "front"}, {Name: "back", Default: true}},
	})

	if step.Kind != flow.StepSelect {
		t.Errorf("Kind = %v, want a single select: run up starts one profile", step.Kind)
	}
	if skip, _ := step.Skip(flow.Answers{}); skip {
		t.Fatal("the step was skipped although two profiles exist")
	}
	content, err := step.Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if content.Start != "back" {
		t.Errorf("Start = %q, want the cursor on the default profile", content.Start)
	}
}

func TestProfileStepResolvesToTheDefaultProfile(t *testing.T) {
	step := target.ProfileStep(target.ProfileParams{
		Profiles: []domain.ProfileConfig{{Name: "default", Default: true}, {Name: "full"}},
	})

	answer, err := step.Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if answer.Value != "default" {
		t.Errorf("Resolve = %q, want the default profile", answer.Value)
	}
}

// Several profiles and none marked default is a required selection with no
// safe default: starting the first declared was a guess nobody could see.
func TestProfileStepRefusesAnUnattendedRunWithNoDefault(t *testing.T) {
	step := target.ProfileStep(target.ProfileParams{
		Profiles: []domain.ProfileConfig{{Name: "front"}, {Name: "back"}},
	})

	_, err := step.Resolve(flow.Answers{})
	if !errors.Is(err, domain.ErrProfileRequired) {
		t.Fatalf("err = %v, want ErrProfileRequired", err)
	}
	if !strings.Contains(err.Error(), "front") || !strings.Contains(err.Error(), "back") {
		t.Errorf("err = %v, want it to name the profiles to choose from", err)
	}
}

func TestDeclaredJobRefusesANameRunTomlDoesNotHold(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}

	if _, err := target.DeclaredJob(cfg, "ghost"); !errors.Is(err, domain.ErrJobNotFound) {
		t.Errorf("err = %v, want ErrJobNotFound", err)
	}
	if _, err := target.DeclaredJob(cfg, ""); !errors.Is(err, domain.ErrJobRequired) {
		t.Errorf("err = %v, want ErrJobRequired", err)
	}
	if job, err := target.DeclaredJob(cfg, "web"); err != nil || job.Name != "web" {
		t.Errorf("DeclaredJob = (%v, %v), want the declaration", job, err)
	}
}

func repoWithSecondWorktree(t *testing.T) (repo, second string) {
	t.Helper()
	repo = gittest.InitRepo(t)
	second = filepath.Join(t.TempDir(), "feature")
	gittest.Git(t, repo, "worktree", "add", "-b", "feature", second)
	return repo, target.Root(second)
}

func badgesByValue(options []flow.Option) map[string]string {
	byValue := make(map[string]string, len(options))
	for _, option := range options {
		texts := make([]string, 0, len(option.Badges))
		for _, badge := range option.Badges {
			texts = append(texts, badge.Text)
		}
		byValue[option.Value] = strings.Join(texts, " ")
	}
	return byValue
}

func TestWorktreesStepResolvesToTheCurrentWorktreeAsASetOfOne(t *testing.T) {
	repo := gittest.InitRepo(t)

	step := target.WorktreesStep(target.WorktreesParams{ProjectDir: repo, Current: repo})

	answer, err := step.Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(answer.Values) != 1 || answer.Values[0] != target.Root(repo) {
		t.Errorf("Resolve = %v, want the current worktree alone", answer.Values)
	}
}

// Nothing named means the run would act where it stands, so the list opens with
// that row already checked rather than with an empty set to arm.
func TestWorktreesStepPrechecksWhatTheRunWouldActOnAnyway(t *testing.T) {
	repo, second := repoWithSecondWorktree(t)

	step := target.WorktreesStep(target.WorktreesParams{ProjectDir: repo, Current: repo})

	content, err := step.Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, option := range content.Options {
		want := option.Value == target.Root(repo)
		if option.Selected != want {
			t.Errorf("%q selected = %v, want %v", option.Value, option.Selected, want)
		}
	}
	if second == "" {
		t.Fatal("the second worktree was never created")
	}
}

func TestWorktreesStepPrechecksThePositionalsInstead(t *testing.T) {
	repo, second := repoWithSecondWorktree(t)

	step := target.WorktreesStep(target.WorktreesParams{
		ProjectDir: repo,
		Current:    repo,
		Selected:   []string{second},
	})

	content, err := step.Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, option := range content.Options {
		want := option.Value == second
		if option.Selected != want {
			t.Errorf("%q selected = %v, want only what the positionals named", option.Value, option.Selected)
		}
	}
}

// A run with no worktree is a run with nothing to do, never a run on all of them.
func TestWorktreesStepRefusesAnEmptySelection(t *testing.T) {
	repo := gittest.InitRepo(t)

	step := target.WorktreesStep(target.WorktreesParams{ProjectDir: repo, Current: repo})

	if err := step.ValidateSet(nil); err == nil {
		t.Fatal("an empty selection was accepted")
	}
	if err := step.ValidateSet([]string{repo}); err != nil {
		t.Errorf("ValidateSet: %v", err)
	}
}

func TestWorktreesStepSummarizesPathsAsBranches(t *testing.T) {
	repo, second := repoWithSecondWorktree(t)

	step := target.WorktreesStep(target.WorktreesParams{ProjectDir: repo, Current: repo})

	got := step.Summarize(flow.Answer{Values: []string{target.Root(repo), second}})
	if !strings.Contains(got, "feature") {
		t.Errorf("Summarize = %q, want the branch names", got)
	}
}

func TestWorkDirsPrefersTheAnswerThenThePositionalsThenTheCwd(t *testing.T) {
	repo := gittest.InitRepo(t)

	answered := target.WorkDirs(target.WorkDirsParams{
		Answers: flow.Answers{}.WithValues(target.KeyWorktree, []string{"/a", "/b"}),
		Named:   []target.Resolved{{Dir: "/named"}},
		Cwd:     repo,
	})
	if len(answered) != 2 || answered[0] != "/a" || answered[1] != "/b" {
		t.Errorf("WorkDirs = %v, want what the step answered", answered)
	}

	named := target.WorkDirs(target.WorkDirsParams{
		Named: []target.Resolved{{Dir: "/named"}, {Dir: "/other"}},
		Cwd:   repo,
	})
	if len(named) != 2 || named[0] != "/named" {
		t.Errorf("WorkDirs = %v, want the positionals", named)
	}

	fallback := target.WorkDirs(target.WorkDirsParams{Cwd: repo})
	if len(fallback) != 1 || fallback[0] != target.Root(repo) {
		t.Errorf("WorkDirs = %v, want the current worktree alone", fallback)
	}
}

// A branch and a path can name the same worktree. Running it twice would race
// two identical sequences, the second failing on the jobs the first started.
func TestWorkDirsKeepsOneMentionOfEachWorktree(t *testing.T) {
	dirs := target.WorkDirs(target.WorkDirsParams{
		Named: []target.Resolved{{Dir: "/a"}, {Dir: "/b"}, {Dir: "/a"}},
		Cwd:   "/cwd",
	})

	if len(dirs) != 2 || dirs[0] != "/a" || dirs[1] != "/b" {
		t.Errorf("WorkDirs = %v, want each worktree once in the order it was named", dirs)
	}
}

// --exclusive stops all but one, so it cannot be applied to a wider selection.
// The picker refuses at the tick rather than after the recap.
func TestWorktreesStepRefusesASecondTickForASingleWorktreeRun(t *testing.T) {
	repo := gittest.InitRepo(t)
	step := target.WorktreesStep(target.WorktreesParams{ProjectDir: repo, Current: repo, Single: true})

	if err := step.ValidateSet([]string{"/a"}); err != nil {
		t.Errorf("ValidateSet: %v", err)
	}
	if err := step.ValidateSet([]string{"/a", "/b"}); !errors.Is(err, domain.ErrExclusiveMultiWorktree) {
		t.Errorf("ValidateSet = %v, want the contradiction refused", err)
	}
}

// A surface that already knows a likely answer offers it, and the step opens
// with those ticked instead of the current worktree. It is only a proposal: the
// selection stays the user's, and nothing is preset.
func TestWorktreesStepOpensOnWhatTheSurfacePrechecked(t *testing.T) {
	repo, second := repoWithSecondWorktree(t)

	step := target.WorktreesStep(target.WorktreesParams{
		ProjectDir: repo,
		Current:    repo,
		Selected:   target.Preselected(target.PreselectedParams{Precheck: []string{second}}),
	})

	content, err := step.Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, option := range content.Options {
		if want := option.Value == second; option.Selected != want {
			t.Errorf("%q selected = %v, want %v — the precheck answers, not the cwd", option.Value, option.Selected, want)
		}
	}
}

// The positionals outrank the proposal: what the command was told beats what the
// surface guessed.
func TestPreselectedPrefersThePositionals(t *testing.T) {
	got := target.Preselected(target.PreselectedParams{
		Named:    []target.Resolved{{Dir: "/named"}},
		Precheck: []string{"/guessed"},
	})

	if len(got) != 1 || got[0] != "/named" {
		t.Errorf("Preselected = %v, want the positionals", got)
	}
}
