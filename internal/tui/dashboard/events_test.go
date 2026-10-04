package dashboard

import (
	"context"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestTheFirstSnapshotIsTheStateTheDashboardAlreadyLoaded(t *testing.T) {
	changed, synced := worktreesChanged(domain.Event{Type: domain.EventSnapshot}, false)
	if changed {
		t.Error("the first snapshot must not reload what Init just loaded")
	}
	if changed, _ := worktreesChanged(domain.Event{Type: domain.EventSnapshot}, synced); !changed {
		t.Error("a later snapshot follows a reconnection, and may carry anything: it must reload")
	}
}

func TestAWorktreeEventChangesTheList(t *testing.T) {
	for _, typ := range []domain.EventType{domain.EventWorktreeCreated, domain.EventWorktreeRemoved, domain.EventWorktreeUpdated} {
		if changed, _ := worktreesChanged(domain.Event{Type: typ}, true); !changed {
			t.Errorf("%s must trigger a reload", typ)
		}
	}
	for _, typ := range []domain.EventType{domain.EventReady, "job.started"} {
		if changed, _ := worktreesChanged(domain.Event{Type: typ}, true); changed {
			t.Errorf("%s must not trigger a reload", typ)
		}
	}
}

func TestABurstOfEventsIsOneSignalAndNeverBlocksTheWatcher(t *testing.T) {
	changes := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	delivered := make(chan struct{})

	go watchEvents(watchEventsParams{
		Context: ctx,
		Changes: changes,
		Watch: func(_ context.Context, onEvent func(domain.Event)) error {
			for range 30 {
				onEvent(domain.Event{Type: domain.EventWorktreeRemoved})
			}
			close(delivered)
			return nil
		},
	})

	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("a pending signal blocked the watcher")
	}
	<-changes
	select {
	case <-changes:
		t.Fatal("thirty events left more than one signal")
	default:
	}
}

func TestReloadsAskedWhileOneIsInFlightCollapseIntoOneMore(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	model, first := model.reload()
	if first == nil {
		t.Fatal("the first reload must run")
	}
	for range 30 {
		next, cmd := model.applyFlow(cleanedMsg{})
		model = next
		if cmd != nil {
			t.Fatal("a reload asked while one is in flight must wait for it")
		}
	}

	model, again := model.reloadLanded()
	if again == nil {
		t.Fatal("what was asked meanwhile must be read by one more reload")
	}
	if _, more := model.reloadLanded(); more != nil {
		t.Fatal("nothing was asked during the second reload: it must be the last")
	}
}
