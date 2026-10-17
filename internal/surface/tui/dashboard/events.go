package dashboard

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/events"
)

type worktreesChangedMsg struct{}

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
	Changes chan<- struct{}
	Watch   WatchFunc
}

// watchEvents never waits on the model: Changes holds one pending signal at
// most, so a burst of events collapses into the single reload that reads them
// all, and none is lost to a channel full of hook output.
func watchEvents(params watchEventsParams) {
	synced := false
	_ = params.Watch(params.Context, func(event domain.Event) {
		changed := false
		changed, synced = worktreesChanged(event, synced)
		if !changed {
			return
		}
		select {
		case params.Changes <- struct{}{}:
		default:
		}
	})
}

// worktreesChanged skips the first snapshot, the state Init already loaded; a
// later one follows a reconnection, during which anything may have changed.
func worktreesChanged(event domain.Event, synced bool) (changed, nowSynced bool) {
	switch {
	case event.Type == domain.EventSnapshot && !synced:
		return false, true
	case event.Type == domain.EventSnapshot, strings.HasPrefix(string(event.Type), domain.EventWorktreePrefix):
		return true, synced
	default:
		return false, synced
	}
}

func awaitChangeCmd(changes <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-changes
		return worktreesChangedMsg{}
	}
}
