package rules_test

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestJobURLFromFlags(t *testing.T) {
	got, err := rules.JobURLFromFlags(rules.JobURLFlagsParams{Port: "PORT", Host: "api.app-1"})
	if err != nil {
		t.Fatalf("JobURLFromFlags: %v", err)
	}
	if got == nil || got.Port != "PORT" || got.Host != "api.app-1" {
		t.Errorf("got %+v, want PORT under api.app-1", got)
	}
}

func TestJobURLFromFlagsPublishesNothingByDefault(t *testing.T) {
	got, err := rules.JobURLFromFlags(rules.JobURLFlagsParams{})
	if err != nil || got != nil {
		t.Errorf("got %+v, %v — a job publishes nothing unless asked", got, err)
	}
}

// A host with nothing to publish is a typo, not a job that publishes under it.
func TestJobURLFromFlagsRefusesAHostWithoutAPort(t *testing.T) {
	_, err := rules.JobURLFromFlags(rules.JobURLFlagsParams{Host: "api.app-1"})
	if err == nil || !strings.Contains(err.Error(), domain.FlagURLPort) {
		t.Errorf("err = %v, want one naming --%s", err, domain.FlagURLPort)
	}
}
