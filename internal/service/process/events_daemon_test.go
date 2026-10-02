package process

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
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
	if _, err := NewClient(d.socket).Send(Request{Action: ActionShutdown}); err != nil {
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

func TestSubscribingToAnOlderDaemonIsAVersionMismatch(t *testing.T) {
	socket := skewSocket(t)
	serveOld(t, socket, nil)
	_, err := Subscribe(context.Background(), SubscribeParams{SocketPath: socket})
	if !errors.Is(err, domain.ErrDaemonVersionMismatch) {
		t.Fatalf("got %v, want ErrDaemonVersionMismatch", err)
	}
}
