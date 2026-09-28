package clean

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

func deferredOffer() bringUpOffer {
	return bringUpOffer{
		Up: map[string]bool{"keycloak": true},
		Result: runjobs.RemoveNamespacesResult{Deferred: []domain.NamespaceRef{
			{Job: "postgres", Worktree: "feat-x"},
			// Up but deferred: its remove failed, and starting it again fixes nothing.
			{Job: "keycloak", Worktree: "feat-x"},
		}},
	}
}

// Starting a service nobody asked for is not a safe default: an unattended
// clean keeps deferring and asks nothing.
func TestAnUnattendedCleanNeverStartsAService(t *testing.T) {
	f := &cleanFlow{prompter: flow.Unattended{}}

	result := f.offerToBringUp(deferredOffer())

	if len(result.Deferred) != 2 {
		t.Errorf("deferred = %+v, want both kept", result.Deferred)
	}
}

// One question per service that was down, none for one whose remove failed
// while up; declining keeps the debt for the service's next start.
func TestACleanAsksOncePerServiceThatWasDown(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{Confirmed: false}
	f := &cleanFlow{prompter: prompter, presenter: nil}

	result := f.offerToBringUp(deferredOffer())

	if prompter.Confirms != 1 {
		t.Errorf("asked %d time(s), want once, for postgres", prompter.Confirms)
	}
	if len(result.Deferred) != 2 {
		t.Errorf("deferred = %+v, want both kept after a no", result.Deferred)
	}
}
