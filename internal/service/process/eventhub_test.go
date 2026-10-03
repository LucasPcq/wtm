package process

import (
	"encoding/json"
	"testing"
)

func TestTheHubDeliversOnlyTheReposASubscriberAskedFor(t *testing.T) {
	hub := newEventHub(4)
	filtered, unsubFiltered := hub.subscribe([]string{"/a"})
	defer unsubFiltered()
	all, unsubAll := hub.subscribe(nil)
	defer unsubAll()

	hub.publish(eventHubPublishParams{Repo: "/b", Payload: json.RawMessage(`1`)})

	if got := len(filtered.ch); got != 0 {
		t.Fatalf("a subscriber filtered on /a received %d events for /b", got)
	}
	if got := len(all.ch); got != 1 {
		t.Fatalf("an unfiltered subscriber received %d events, want 1", got)
	}
}

func TestAFullQueueDisconnectsItsSubscriberWithoutBlocking(t *testing.T) {
	hub := newEventHub(1)
	slow, unsub := hub.subscribe(nil)
	defer unsub()

	hub.publish(eventHubPublishParams{Repo: "/a", Payload: json.RawMessage(`1`)})
	hub.publish(eventHubPublishParams{Repo: "/a", Payload: json.RawMessage(`2`)})

	<-slow.ch
	if _, open := <-slow.ch; open {
		t.Fatal("a subscriber that fell behind must be disconnected")
	}
}

func TestClosingTheHubEndsEverySubscriber(t *testing.T) {
	hub := newEventHub(4)
	sub, unsub := hub.subscribe(nil)
	hub.close()
	if _, open := <-sub.ch; open {
		t.Fatal("closing the hub must close its subscribers")
	}
	unsub()
	hub.publish(eventHubPublishParams{Repo: "/a", Payload: json.RawMessage(`1`)})

	late, _ := hub.subscribe(nil)
	if _, open := <-late.ch; open {
		t.Fatal("a subscription to a closed hub must end at once")
	}
}
