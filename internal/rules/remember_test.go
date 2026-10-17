package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestValidateRememberedAcceptsEveryOfferedAnswer(t *testing.T) {
	remembered := map[string]string{
		domain.RememberEnvStrategy:  "parent",
		domain.RememberIsolation:    "verbatim",
		domain.RememberSourceUpdate: "keep",
	}
	if err := ValidateRemembered(remembered); err != nil {
		t.Errorf("ValidateRemembered: %v", err)
	}
}

func TestValidateRememberedRefusesAnUnknownQuestion(t *testing.T) {
	err := ValidateRemembered(map[string]string{"delete": "force"})
	if err == nil || !strings.Contains(err.Error(), `"delete"`) {
		t.Errorf("err = %v, want the unknown id named", err)
	}
}

func TestValidateRememberedRefusesAValueTheQuestionNeverOffers(t *testing.T) {
	err := ValidateRemembered(map[string]string{domain.RememberIsolation: "shared"})
	if err == nil || !strings.Contains(err.Error(), "isolated, verbatim") {
		t.Errorf("err = %v, want the allowed values named", err)
	}
}

func TestApplyRememberedForgetsThenKeeps(t *testing.T) {
	next := ApplyRemembered(
		map[string]string{domain.RememberEnvStrategy: "parent", domain.RememberIsolation: "verbatim"},
		RememberedChange{Remember: map[string]string{domain.RememberSourceUpdate: "ff"}, Forget: []string{domain.RememberIsolation}},
	)
	want := map[string]string{domain.RememberEnvStrategy: "parent", domain.RememberSourceUpdate: "ff"}
	if len(next) != len(want) || next[domain.RememberEnvStrategy] != "parent" || next[domain.RememberSourceUpdate] != "ff" {
		t.Errorf("next = %v, want %v", next, want)
	}
}

func TestApplyRememberedLeavesNothingWhenEverythingIsForgotten(t *testing.T) {
	next := ApplyRemembered(map[string]string{domain.RememberIsolation: "verbatim"}, RememberedChange{Forget: []string{domain.RememberIsolation}})
	if next != nil {
		t.Errorf("next = %v, want nil so the section leaves the file", next)
	}
}
