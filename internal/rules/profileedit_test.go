package rules_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func boolPtr(b bool) *bool { return &b }

func fullProfile() domain.ProfileConfig {
	return domain.ProfileConfig{Name: "dev", Jobs: []string{"db", "api"}, Default: true}
}

func TestApplyProfilePatchLeavesEveryUntouchedField(t *testing.T) {
	got, err := rules.ApplyProfilePatch(rules.ApplyProfilePatchParams{
		Current: fullProfile(),
		Patch:   rules.ProfilePatch{Jobs: []string{"api", "web"}},
	})
	if err != nil {
		t.Fatalf("ApplyProfilePatch: %v", err)
	}
	if !slices.Equal(got.Jobs, []string{"api", "web"}) {
		t.Errorf("jobs = %v, want the list replaced in the given order", got.Jobs)
	}
	if got.Name != "dev" || !got.Default {
		t.Errorf("got %+v, want name and default untouched", got)
	}
}

func TestApplyProfilePatchUnsetsDefault(t *testing.T) {
	got, err := rules.ApplyProfilePatch(rules.ApplyProfilePatchParams{
		Current: fullProfile(),
		Patch:   rules.ProfilePatch{Default: boolPtr(false)},
	})
	if err != nil {
		t.Fatalf("ApplyProfilePatch: %v", err)
	}
	if got.Default {
		t.Error("default = true, want it unset by --default=false")
	}
	if !slices.Equal(got.Jobs, []string{"db", "api"}) {
		t.Errorf("jobs = %v, want them untouched", got.Jobs)
	}
}

func TestApplyProfilePatchRenames(t *testing.T) {
	got, err := rules.ApplyProfilePatch(rules.ApplyProfilePatchParams{
		Current: fullProfile(),
		Patch:   rules.ProfilePatch{Name: ptr("front")},
	})
	if err != nil {
		t.Fatalf("ApplyProfilePatch: %v", err)
	}
	if got.Name != "front" {
		t.Errorf("name = %q, want front", got.Name)
	}
}

func TestApplyProfilePatchRefusesEmptyValues(t *testing.T) {
	tests := []struct {
		name  string
		patch rules.ProfilePatch
		flag  string
	}{
		{name: "name", patch: rules.ProfilePatch{Name: ptr(" ")}, flag: domain.FlagName},
		{name: "jobs", patch: rules.ProfilePatch{Jobs: []string{" ", ""}}, flag: domain.FlagJobs},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rules.ApplyProfilePatch(rules.ApplyProfilePatchParams{Current: fullProfile(), Patch: tt.patch})
			if err == nil || !strings.Contains(err.Error(), tt.flag) {
				t.Errorf("err = %v, want one naming --%s", err, tt.flag)
			}
		})
	}
}

func TestProfilePatchEmpty(t *testing.T) {
	if !(rules.ProfilePatch{}).Empty() {
		t.Error("a zero patch changes nothing")
	}
	if (rules.ProfilePatch{Default: boolPtr(false)}).Empty() {
		t.Error("unsetting default is a change")
	}
}
