package dashboard

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/events"
)

type worktreeEventMsg struct {
	event domain.Event
}

// WatchFunc streams a repository's events to onEvent until ctx is done.
type WatchFunc func(ctx context.Context, onEvent func(domain.Event)) error

func defaultWatch(params RunParams) WatchFunc {
	return func(ctx context.Context, onEvent func(domain.Event)) error {
		return events.Watch(ctx, events.WatchParams{
			ProjectDir: params.ProjectDir,
			StateDir:   params.StateDir,
			ProxyPort:  rules.ProxyPort(params.Config.Global),
			OnEvent: func(received events.Received) error {
				onEvent(received.Event)
				return nil
			},
		})
	}
}

type watchEventsParams struct {
	Context context.Context
	Msgs    chan<- tea.Msg
	Watch   WatchFunc
}

// watchEvents never waits on the model: a full channel drops the event, and the
// reload already queued reads whatever it described.
func watchEvents(params watchEventsParams) {
	_ = params.Watch(params.Context, func(event domain.Event) {
		select {
		case params.Msgs <- worktreeEventMsg{event: event}:
		default:
		}
	})
}

// applyEvent reloads on any change to a worktree. The first snapshot is the
// state Init already loaded; a later one follows a reconnection, during which
// anything may have changed.
func (m Model) applyEvent(event domain.Event) (Model, tea.Cmd) {
	switch {
	case event.Type == domain.EventSnapshot && !m.eventsSynced:
		m.eventsSynced = true
		return m, nil
	case event.Type == domain.EventSnapshot, strings.HasPrefix(string(event.Type), domain.EventWorktreePrefix):
		return m, m.reload()
	default:
		return m, nil
	}
}
