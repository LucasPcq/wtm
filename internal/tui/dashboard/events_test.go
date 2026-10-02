package dashboard

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestTheFirstSnapshotIsTheStateTheDashboardAlreadyLoaded(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	model, cmd := model.applyFlow(worktreeEventMsg{event: domain.Event{Type: domain.EventSnapshot}})
	if cmd != nil {
		t.Error("the first snapshot must not reload what Init just loaded")
	}
	if _, cmd := model.applyFlow(worktreeEventMsg{event: domain.Event{Type: domain.EventSnapshot}}); cmd == nil {
		t.Error("a later snapshot follows a reconnection, and may carry anything: it must reload")
	}
}

func TestAWorktreeEventReloadsTheList(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	for _, typ := range []domain.EventType{domain.EventWorktreeCreated, domain.EventWorktreeRemoved, domain.EventWorktreeUpdated} {
		if _, cmd := model.applyFlow(worktreeEventMsg{event: domain.Event{Type: typ}}); cmd == nil {
			t.Errorf("%s must trigger a reload", typ)
		}
	}
	for _, typ := range []domain.EventType{domain.EventReady, "job.started"} {
		if _, cmd := model.applyFlow(worktreeEventMsg{event: domain.Event{Type: typ}}); cmd != nil {
			t.Errorf("%s must not trigger a reload", typ)
		}
	}
}

func TestEventsReachTheModelWithoutEverBlockingTheWatcher(t *testing.T) {
	msgs := make(chan tea.Msg, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	delivered := make(chan struct{})

	go watchEvents(watchEventsParams{
		Context: ctx,
		Msgs:    msgs,
		Watch: func(_ context.Context, onEvent func(domain.Event)) error {
			for range 3 {
				onEvent(domain.Event{Type: domain.EventWorktreeCreated})
			}
			close(delivered)
			return nil
		},
	})

	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("a full channel blocked the watcher")
	}
	if msg, ok := (<-msgs).(worktreeEventMsg); !ok || msg.event.Type != domain.EventWorktreeCreated {
		t.Fatalf("got %+v", msg)
	}
}
