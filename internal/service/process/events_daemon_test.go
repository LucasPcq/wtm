package process

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/socktest"
)

func TestAPublishedEventReachesASubscriberOfItsRepo(t *testing.T) {
	d := idleDaemon(t, time.Hour, daemonNamespaceBudget)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deliveries, err := Subscribe(ctx, SubscribeParams{SocketPath: d.socket, Repos: []string{"/repo/.git"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := Publish(PublishParams{SocketPath: d.socket, Repo: "/repo/.git", Payload: json.RawMessage(`{"type":"x"}`)}); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-deliveries:
		if string(got.Payload) != `{"type":"x"}` || got.Repo != "/repo/.git" {
			t.Fatalf("got %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no delivery")
	}
}

func TestPublishingWithNoDaemonReturnsAtOnce(t *testing.T) {
	start := time.Now()
	err := Publish(PublishParams{SocketPath: skewSocket(t), Repo: "/r", Payload: json.RawMessage(`{}`)})
	if err == nil {
		t.Fatal("publishing to no daemon must report it to the caller, who drops it")
	}
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("publish took %s with no daemon", elapsed)
	}
}

func TestShuttingDownEndsSubscribersAndTheDaemon(t *testing.T) {
	d := idleDaemon(t, time.Hour, daemonNamespaceBudget)
	deliveries, err := Subscribe(context.Background(), SubscribeParams{SocketPath: d.socket})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewClient(d.socket).Send(t.Context(), Request{Action: ActionShutdown}); err != nil {
		t.Fatal(err)
	}
	select {
	case _, open := <-deliveries:
		if open {
			t.Fatal("expected the stream to end")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the subscription outlived the daemon")
	}
	select {
	case <-d.exited:
	case <-time.After(3 * time.Second):
		t.Fatal("RunDaemon did not return with a subscriber connected")
	}
}

func TestASubscriberKeepsTheDaemonAliveUntilItLeaves(t *testing.T) {
	d := idleDaemon(t, 200*time.Millisecond, daemonNamespaceBudget)
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := Subscribe(ctx, SubscribeParams{SocketPath: d.socket}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-d.exited:
		t.Fatal("a connected subscriber must keep the daemon alive")
	case <-time.After(600 * time.Millisecond):
	}
	cancel()
	select {
	case <-d.exited:
	case <-time.After(3 * time.Second):
		t.Fatal("a departed subscriber kept the daemon alive")
	}
}

func TestADaemonThatPredatesSubscribeSaysSo(t *testing.T) {
	socket := skewSocket(t)
	serveOld(t, socket, nil)
	_, err := Subscribe(context.Background(), SubscribeParams{SocketPath: socket})
	if !errors.Is(err, domain.ErrDaemonNoSubscribe) {
		t.Fatalf("got %v, want ErrDaemonNoSubscribe", err)
	}
}

// The broker never reads what it relays, so a daemon of another build that
// knows subscribe serves a subscriber as well as its own: refusing it is what
// set two watchers of two builds replacing each other's daemon in a loop.
func TestADaemonOfAnotherBuildThatKnowsSubscribeIsAccepted(t *testing.T) {
	socket := skewSocket(t)
	socktest.Serve(t, socket, func(json.RawMessage) any {
		return Response{Status: StatusOK, Version: "v0.0.0-other"}
	})
	if _, err := Subscribe(context.Background(), SubscribeParams{SocketPath: socket}); err != nil {
		t.Fatalf("Subscribe = %v, want the other build accepted", err)
	}
}

func TestTheDaemonPublishesAJobsLifecycleUnderItsOrigin(t *testing.T) {
	d := idleDaemon(t, time.Hour, daemonNamespaceBudget)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deliveries, err := Subscribe(ctx, SubscribeParams{SocketPath: d.socket, Repos: []string{"/code/app/.git"}})
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	origin := &domain.EventOrigin{Repo: domain.EventRepo{Root: "/code/app", CommonDir: "/code/app/.git"}, CorrelationID: "popup-7"}
	job := domain.JobConfig{Name: "web", Kind: domain.JobKindService, Cmd: "echo 'Error: boom'; exit 4"}
	resp, err := NewClient(d.socket).Send(t.Context(), Request{Action: ActionStart, Job: &job, WorkDir: dir, Env: map[string]string{domain.EnvBranch: "feat/a"}, Origin: origin})
	if err != nil || resp.Status != StatusOK {
		t.Fatalf("start: %+v, %v", resp, err)
	}

	started := nextJobEvent(t, deliveries)
	crashed := nextJobEvent(t, deliveries)
	if started.Type != domain.EventJobStarted || crashed.Type != domain.EventJobCrashed {
		t.Fatalf("heard %s then %s", started.Type, crashed.Type)
	}
	want := domain.WorktreeRef{Branch: "feat/a", Path: dir}
	if crashed.Worktree != want || crashed.Repo != origin.Repo || crashed.Job.Name != "web" || crashed.Job.Kind != domain.JobKindService {
		t.Fatalf("crashed = %+v", crashed)
	}
	if crashed.V != domain.EventsSchemaVersion || crashed.TS == "" || crashed.CorrelationID != "popup-7" {
		t.Fatalf("crashed is stamped %d %q %q", crashed.V, crashed.TS, crashed.CorrelationID)
	}
	if crashed.ExitCode == nil || *crashed.ExitCode != 4 || len(crashed.LastLines) == 0 {
		t.Fatalf("crashed carries exit %v and lines %q", crashed.ExitCode, crashed.LastLines)
	}
}

func TestAJobStartedWithoutAnOriginPublishesNothing(t *testing.T) {
	d := idleDaemon(t, time.Hour, daemonNamespaceBudget)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deliveries, err := Subscribe(ctx, SubscribeParams{SocketPath: d.socket})
	if err != nil {
		t.Fatal(err)
	}
	job := domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask, Cmd: "true"}
	if _, err := NewClient(d.socket).Send(t.Context(), Request{Action: ActionStart, Job: &job, WorkDir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-deliveries:
		t.Fatalf("published %s for a job nobody can place in a repository", got.Payload)
	case <-time.After(300 * time.Millisecond):
	}
}

func nextJobEvent(t *testing.T, deliveries <-chan Delivery) domain.JobEvent {
	t.Helper()
	select {
	case delivery := <-deliveries:
		var event domain.JobEvent
		if err := json.Unmarshal(delivery.Payload, &event); err != nil {
			t.Fatal(err)
		}
		if delivery.Repo != event.Repo.CommonDir {
			t.Fatalf("delivered under %q, want the event's repository", delivery.Repo)
		}
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("no job event")
		return domain.JobEvent{}
	}
}
