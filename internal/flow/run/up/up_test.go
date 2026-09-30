package up

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// A flag that contradicts itself is not a decision to default: --exclusive
// stops all but one, and the run was told to bring up two.
func TestExclusiveIsRefusedOnSeveralWorktrees(t *testing.T) {
	repo := gittest.InitRepo(t)
	second := filepath.Join(t.TempDir(), "feature")
	gittest.Git(t, repo, "worktree", "add", "-b", "feature", second)

	_, err := Run(Params{
		Context:   flow.Context{ProjectDir: repo},
		Request:   Request{Worktrees: []string{"main", "feature"}, Cwd: repo, Exclusive: true},
		Prompter:  flow.Unattended{},
		Presenter: presenterOnly{&flowtest.Recorder{}},
	})

	if !errors.Is(err, domain.ErrExclusiveMultiWorktree) {
		t.Fatalf("err = %v, want the contradiction refused", err)
	}
}

func TestTheNamedProfileIsTheOneStarted(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "db", Kind: domain.JobKindService},
			{Name: "web", Kind: domain.JobKindService},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "front", Jobs: []string{"db", "web"}, Default: true},
			{Name: "back", Jobs: []string{"db"}},
		},
	}
	f := &upFlow{request: Request{Config: cfg}}

	got, err := f.resolveProfile(flow.NewAnswers(map[string]string{target.KeyProfile: "back"}))
	if err != nil {
		t.Fatalf("resolveProfile: %v", err)
	}
	if got.Name != "back" || len(got.Jobs) != 1 {
		t.Errorf("got %+v, want back alone", got)
	}
}

func TestTheOnlyProfileIsStartedWithoutBeingNamed(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}},
		Profiles: []domain.ProfileConfig{{Name: "front", Jobs: []string{"web"}}},
	}
	f := &upFlow{request: Request{Config: cfg}}

	got, err := f.resolveProfile(flow.Answers{})
	if err != nil {
		t.Fatalf("resolveProfile: %v", err)
	}
	if got.Name != "front" || len(got.Jobs) != 1 {
		t.Errorf("got %+v, want the only profile", got)
	}
}

func TestNoProfileDeclaredStartsEveryJob(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{{Name: "migrate", Kind: domain.JobKindTask}, {Name: "web", Kind: domain.JobKindService}},
	}
	f := &upFlow{request: Request{Config: cfg}}

	got, err := f.resolveProfile(flow.Answers{})
	if err != nil {
		t.Fatalf("resolveProfile: %v", err)
	}
	if got.Name != "" || len(got.Jobs) != 2 {
		t.Errorf("got %+v, want every declared job under no profile name", got)
	}
}

func TestAnUnattendedRunRefusesSeveralProfilesWithNoDefault(t *testing.T) {
	repo := gittest.InitRepo(t)
	cfg := domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}},
		Profiles: []domain.ProfileConfig{{Name: "front", Jobs: []string{"web"}}, {Name: "back", Jobs: []string{"web"}}},
	}
	f := &upFlow{ctx: flow.Context{ProjectDir: repo}, request: Request{Cwd: repo, Config: cfg}}

	_, err := flow.Unattended{}.Ask(f.session())

	if !errors.Is(err, domain.ErrProfileRequired) {
		t.Fatalf("err = %v, want the run refused naming --profile", err)
	}
}
