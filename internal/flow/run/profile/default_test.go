package profile_test

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	profileflow "github.com/LucasPcq/wtm/internal/flow/run/profile"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
)

func withProfiles(t *testing.T, profiles ...domain.ProfileConfig) (flow.Context, domain.RunConfig) {
	t.Helper()
	ctx := flow.Context{StateDir: t.TempDir()}
	cfg := declared()
	cfg.Profiles = profiles
	if err := runconfig.Save(runconfig.SaveParams{StateDir: ctx.StateDir, Config: cfg}); err != nil {
		t.Fatalf("save: %v", err)
	}
	return ctx, cfg
}

func warned(r *recorder, parts ...string) bool {
	for _, notice := range r.Notices {
		if notice.Kind != flow.NoticeWarning {
			continue
		}
		all := true
		for _, part := range parts {
			all = all && strings.Contains(notice.Text, part)
		}
		if all {
			return true
		}
	}
	return false
}

func TestAddingANewDefaultSaysWhichOneItReplaced(t *testing.T) {
	ctx, cfg := withProfiles(t, domain.ProfileConfig{Name: "full", Jobs: []string{"api", "web"}, Default: true})
	rec := &recorder{}

	_, err := profileflow.Add(profileflow.AddParams{
		Context:   ctx,
		Request:   profileflow.AddRequest{Initial: domain.ProfileConfig{Name: "api-only", Jobs: []string{"api"}, Default: true}, Config: cfg},
		Prompter:  flow.Unattended{},
		Presenter: rec,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !warned(rec, "full", "api-only") {
		t.Errorf("notices = %+v, want a warning naming full and api-only", rec.Notices)
	}
}

func TestEditingTheDefaultOntoItselfSaysNothing(t *testing.T) {
	ctx, cfg := withProfiles(t, domain.ProfileConfig{Name: "full", Jobs: []string{"api", "web"}, Default: true})
	rec := &recorder{}

	_, err := profileflow.Edit(profileflow.EditParams{
		Context:   ctx,
		Request:   profileflow.EditRequest{Name: "full", Patch: patchJobs("api"), Config: cfg},
		Prompter:  flow.Unattended{},
		Presenter: rec,
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if len(rec.Notices) != 0 {
		t.Errorf("notices = %+v, want none", rec.Notices)
	}
}

func TestRemovingTheDefaultSaysWhatRunUpStartsNow(t *testing.T) {
	ctx, cfg := withProfiles(t,
		domain.ProfileConfig{Name: "api-only", Jobs: []string{"api"}},
		domain.ProfileConfig{Name: "full", Jobs: []string{"api", "web"}, Default: true},
	)
	rec := &recorder{}

	_, err := profileflow.Remove(profileflow.RemoveParams{
		Context:   ctx,
		Request:   profileflow.RemoveRequest{Name: "full", Config: cfg},
		Prompter:  flow.Unattended{},
		Presenter: rec,
	})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !warned(rec, "full", "api-only") {
		t.Errorf("notices = %+v, want a warning naming full and the fallback api-only", rec.Notices)
	}
}

func TestRemovingTheLastProfileSaysNothingAboutADefault(t *testing.T) {
	ctx, cfg := withProfiles(t, domain.ProfileConfig{Name: "full", Jobs: []string{"api"}, Default: true})
	rec := &recorder{}

	_, err := profileflow.Remove(profileflow.RemoveParams{
		Context:   ctx,
		Request:   profileflow.RemoveRequest{Name: "full", Config: cfg},
		Prompter:  flow.Unattended{},
		Presenter: rec,
	})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(rec.Notices) != 0 {
		t.Errorf("notices = %+v, want none", rec.Notices)
	}
}

func patchJobs(jobs ...string) rules.ProfilePatch {
	return rules.ProfilePatch{Jobs: jobs}
}
