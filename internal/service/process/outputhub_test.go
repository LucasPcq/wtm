package process

import (
	"testing"
	"time"
)

const hubTestCapacity = 4096

func receive(t *testing.T, ch <-chan []byte) []byte {
	t.Helper()
	select {
	case data, ok := <-ch:
		if !ok {
			t.Fatal("subscriber channel closed while a chunk was expected")
		}
		return data
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for a chunk")
		return nil
	}
}

func TestOutputHubFansOutToEverySubscriber(t *testing.T) {
	hub := newOutputHub(hubTestCapacity)

	_, first, unsubFirst, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe first: %v", err)
	}
	defer unsubFirst()

	_, second, unsubSecond, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe second: %v", err)
	}
	defer unsubSecond()

	if _, err := hub.Write([]byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := string(receive(t, first)); got != "hello" {
		t.Errorf("first subscriber got %q, want %q", got, "hello")
	}
	if got := string(receive(t, second)); got != "hello" {
		t.Errorf("second subscriber got %q, want %q", got, "hello")
	}
}

func TestOutputHubReplaysHistoryOnSubscribe(t *testing.T) {
	hub := newOutputHub(hubTestCapacity)
	hub.Write([]byte("already there"))

	history, _, unsub, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsub()

	if string(history) != "already there" {
		t.Errorf("history = %q, want %q", history, "already there")
	}
}

func TestOutputHubSlowSubscriberDoesNotBlockOthers(t *testing.T) {
	hub := newOutputHub(hubTestCapacity)

	_, slow, unsubSlow, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe slow: %v", err)
	}
	defer unsubSlow()

	_, fast, unsubFast, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe fast: %v", err)
	}
	defer unsubFast()

	// The slow subscriber never reads: its queue overflows and its chunks are
	// dropped, while the fast one keeps seeing every write.
	for i := 0; i < outputSubscriberQueue*2; i++ {
		if _, err := hub.Write([]byte("chunk")); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		if got := string(receive(t, fast)); got != "chunk" {
			t.Fatalf("fast subscriber got %q at write %d, want %q", got, i, "chunk")
		}
	}

	if len(slow) != outputSubscriberQueue {
		t.Errorf("slow subscriber queue holds %d chunks, want it capped at %d", len(slow), outputSubscriberQueue)
	}
}

func TestOutputHubUnsubscribeLeavesTheOtherSubscriber(t *testing.T) {
	hub := newOutputHub(hubTestCapacity)

	_, leaving, unsubLeaving, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe leaving: %v", err)
	}
	_, staying, unsubStaying, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe staying: %v", err)
	}
	defer unsubStaying()

	unsubLeaving()
	if _, ok := <-leaving; ok {
		t.Error("unsubscribing should close that subscriber's channel")
	}

	hub.Write([]byte("still flowing"))
	if got := string(receive(t, staying)); got != "still flowing" {
		t.Errorf("remaining subscriber got %q, want %q", got, "still flowing")
	}

	// A second release, and one after the hub closed, must stay harmless.
	unsubLeaving()
	hub.close()
	unsubLeaving()
}

func TestOutputHubCloseWakesEverySubscriber(t *testing.T) {
	hub := newOutputHub(hubTestCapacity)

	_, first, unsubFirst, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe first: %v", err)
	}
	defer unsubFirst()
	_, second, unsubSecond, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe second: %v", err)
	}
	defer unsubSecond()

	hub.close()

	if _, ok := <-first; ok {
		t.Error("close should close the first subscriber's channel")
	}
	if _, ok := <-second; ok {
		t.Error("close should close the second subscriber's channel")
	}
	if _, _, _, err := hub.Subscribe(); err == nil {
		t.Error("subscribing to a closed hub should fail")
	}
}
