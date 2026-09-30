package owed

import (
	"os"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

// A service the daemon refuses to start takes nothing back: its data stays
// owed, and the refusal is what the reader is told.
func TestDropperDefersWhatAServiceTheDaemonRefusedHolds(t *testing.T) {
	ctx, witness := holdingFixture(t)
	daemon := processtest.Serve(t, nil)
	daemon.StartError = "port 5432 already in use"
	snapshot := readHolding(t, ctx, false)
	presenter := &flowtest.Recorder{}

	dropper := NewDropper(DropperParams{Context: ctx, Presenter: presenter, Snapshot: snapshot, StartDown: true})
	outcomes := dropper.Drop("feat-live")
	dropper.Close()

	if body, _ := os.ReadFile(witness); len(body) != 0 {
		t.Errorf("remove ran with %q against a service that never started", body)
	}
	if len(presenter.Statuses) == 0 || !strings.Contains(presenter.Statuses[0].Text, "could not start postgres") {
		t.Errorf("statuses = %+v, want the refusal said", presenter.Statuses)
	}
	if len(outcomes) != 1 || outcomes[0].Status != domain.NamespaceDeferred {
		t.Errorf("outcomes = %+v, want it deferred", outcomes)
	}
	for _, action := range daemon.Actions() {
		if strings.HasPrefix(action, "stop:") {
			t.Errorf("requests = %v: a service never started has nothing to let go of", daemon.Actions())
		}
	}
}
