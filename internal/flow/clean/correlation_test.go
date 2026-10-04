package clean

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/events"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/socktest"
)

type correlationCase struct {
	ID   string
	Want int
}

// Every event of one run goes out through the one stamped publisher, so the
// children a removal reparents carry the caller's id as the removals do.
func TestEveryEventOfARunCarriesTheCallersCorrelationID(t *testing.T) {
	for _, tc := range []correlationCase{{ID: "x"}, {ID: ""}} {
		t.Run("id="+tc.ID, func(t *testing.T) {
			ctx := testContext(t)
			makeWorktree(t, ctx, "top")
			makeWorktreeFrom(t, ctx, "leaf", "top")
			processtest.Home(t)
			socket := socktest.Path(t)
			processtest.RealDaemon(t, socket)
			received := watchAfterReady(t, watchParams{ProjectDir: ctx.ProjectDir, StateDir: ctx.StateDir, Socket: socket})
			ctx.Publisher = events.NewPublisher(events.PublisherParams{ProjectDir: ctx.ProjectDir, SocketPath: socket, CorrelationID: tc.ID})

			if _, err := Run(Params{
				Context:   ctx,
				Request:   Request{Branches: []string{"top"}, BaseBranch: "main", ReparentChildren: true},
				Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteYes}},
				Presenter: newRecorder(),
			}); err != nil {
				t.Fatalf("Run: %v", err)
			}

			seen := map[domain.EventType]bool{}
			for !seen[domain.EventWorktreeRemoved] || !seen[domain.EventWorktreeReparented] {
				select {
				case r := <-received:
					seen[r.Event.Type] = true
					if r.Event.CorrelationID != tc.ID {
						t.Fatalf("%s carries %q, want %q", r.Event.Type, r.Event.CorrelationID, tc.ID)
					}
					if tc.ID == "" && bytes.Contains(r.Raw, []byte("correlation_id")) {
						t.Fatalf("%s carries the field without an id: %s", r.Event.Type, r.Raw)
					}
				case <-time.After(5 * time.Second):
					t.Fatalf("only saw %v", seen)
				}
			}
		})
	}
}

type watchParams struct {
	ProjectDir string
	StateDir   string
	Socket     string
}

func watchAfterReady(t *testing.T, params watchParams) <-chan events.Received {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	out := make(chan events.Received, 32)
	ready := make(chan struct{})
	go func() {
		_ = events.Watch(ctx, events.WatchParams{
			ProjectDir: params.ProjectDir,
			StateDir:   params.StateDir,
			SocketPath: params.Socket,
			OnEvent: func(r events.Received) error {
				switch r.Event.Type {
				case domain.EventReady:
					close(ready)
				case domain.EventSnapshot:
				default:
					out <- r
				}
				return nil
			},
		})
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("no ready")
	}
	return out
}
