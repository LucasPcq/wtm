package proxy

import (
	"errors"
	"testing"
	"time"
)

func TestCachedAsksAgainOnceTheTTLIsGone(t *testing.T) {
	calls := 0
	target := Cached(func() (int, error) {
		calls++
		return 4000 + calls, nil
	}, 30*time.Millisecond)

	first, _ := target()
	again, _ := target()
	if first != 4001 || again != 4001 {
		t.Errorf("dans la fenêtre, une seule question : got %d puis %d", first, again)
	}

	time.Sleep(40 * time.Millisecond)
	if after, _ := target(); after != 4002 {
		t.Errorf("la fenêtre passée, on redemande : got %d", after)
	}
}

func TestCachedDoesNotCacheAFailure(t *testing.T) {
	calls := 0
	target := Cached(func() (int, error) {
		calls++
		return 0, errors.New("no daemon")
	}, time.Hour)

	_, _ = target()
	_, _ = target()
	if calls != 2 {
		t.Errorf("un échec ne se mémorise pas, sinon le daemon qui revient n'est jamais vu : got %d appels", calls)
	}
}

func TestForwardRefusesWithoutATarget(t *testing.T) {
	if err := Forward(ForwardParams{Listeners: nil, Target: nil}); err == nil {
		t.Error("un forwarder sans socket ni cible doit refuser")
	}
}
