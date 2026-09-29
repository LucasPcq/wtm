package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func touchingConfig() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "pg", Kind: domain.JobKindService, Cmd: "docker compose up -d pg", Scope: domain.JobScopeShared,
			Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "true"}},
		{Name: "minio", Kind: domain.JobKindService, Cmd: "docker compose up -d minio", Scope: domain.JobScopeShared},
		{Name: "reset", Kind: domain.JobKindTask, Cmd: "pnpm reset", Touches: []string{"pg"}},
		{Name: "wipe", Kind: domain.JobKindTask, Cmd: "pnpm wipe", Touches: []string{"minio"}},
		{Name: "build", Kind: domain.JobKindTask, Cmd: "pnpm build"},
	}}
}

func TestForeignDataRisks(t *testing.T) {
	cfg := touchingConfig()
	cases := []struct {
		name      string
		isolation domain.Isolation
		main      bool
		want      map[string]domain.DataOwner
	}{
		{name: "isolated: its own slice, but a namespace-less shared service is everyone's", isolation: domain.IsolationIsolated,
			want: map[string]domain.DataOwner{"wipe": domain.DataOwnerEveryone}},
		{name: "verbatim: everything it touches is its source's", isolation: domain.IsolationVerbatim,
			want: map[string]domain.DataOwner{"reset": domain.DataOwnerSource, "wipe": domain.DataOwnerSource}},
		{name: "main owns its data", isolation: domain.IsolationIsolated, main: true, want: map[string]domain.DataOwner{}},
	}
	for _, tc := range cases {
		risks := ForeignDataRisks(ForeignDataParams{Config: cfg, Jobs: cfg.Jobs, WorkDir: "/wt/x", Isolation: tc.isolation, Main: tc.main})
		got := map[string]domain.DataOwner{}
		for _, risk := range risks {
			got[risk.Job] = risk.Owner
		}
		if len(got) != len(tc.want) {
			t.Errorf("%s: risks = %+v, want %v", tc.name, risks, tc.want)
			continue
		}
		for job, owner := range tc.want {
			if got[job] != owner {
				t.Errorf("%s: %s owner = %q, want %q", tc.name, job, got[job], owner)
			}
		}
	}
}

func TestForeignDataLinesNameTheWorktreeOnlyWhenSeveral(t *testing.T) {
	risks := []domain.DataRisk{{Job: "reset", Service: "pg", Owner: domain.DataOwnerSource, WorkDir: "/wt/feat-x"}}
	single := ForeignDataLines(ForeignDataLinesParams{Risks: risks})
	several := ForeignDataLines(ForeignDataLinesParams{Risks: risks, Several: true})
	if strings.Contains(single[0], "feat-x") || !strings.Contains(several[0], "feat-x") {
		t.Errorf("lines = %q / %q, want the worktree named only over several", single, several)
	}
}

func TestValidateRunRefusesAnUnknownTouches(t *testing.T) {
	cfg := touchingConfig()
	cfg.Jobs[2].Touches = []string{"pgg"}
	_, errs := ValidateRun(cfg)
	if !strings.Contains(strings.Join(errs, "\n"), `touches unknown job "pgg"`) {
		t.Errorf("errs = %v, want the typo refused", errs)
	}
}

func TestRemovingAJobDropsItFromTouches(t *testing.T) {
	out, effect := RemoveJob(touchingConfig(), "pg")
	if len(effect.Touchers) != 1 || effect.Touchers[0] != "reset" {
		t.Errorf("Touchers = %v, want reset", effect.Touchers)
	}
	for _, job := range out.Jobs {
		if job.Name == "reset" && job.Touches != nil {
			t.Errorf("reset touches %v, want nothing left", job.Touches)
		}
	}
	if _, errs := ValidateRun(out); len(errs) != 0 {
		t.Errorf("the removal left an invalid config: %v", errs)
	}
}

func TestRenamingAJobFollowsItsTouches(t *testing.T) {
	cfg := touchingConfig()
	out := RenameJobRefs(cfg, "pg", "postgres")
	for _, job := range out.Jobs {
		if job.Name == "reset" && (len(job.Touches) != 1 || job.Touches[0] != "postgres") {
			t.Errorf("reset touches %v, want the new name", job.Touches)
		}
	}
	if cfg.Jobs[2].Touches[0] != "pg" {
		t.Error("the rename wrote through to the config it was given")
	}
}

func TestJobPatchReplacesTouches(t *testing.T) {
	touches := []string{"minio", ""}
	job, err := ApplyJobPatch(ApplyJobPatchParams{Current: touchingConfig().Jobs[2], Patch: JobPatch{Touches: &touches}})
	if err != nil {
		t.Fatalf("ApplyJobPatch: %v", err)
	}
	if len(job.Touches) != 1 || job.Touches[0] != "minio" {
		t.Errorf("touches = %v, want the list replaced", job.Touches)
	}
}
