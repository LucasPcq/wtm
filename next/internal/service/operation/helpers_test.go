package operation

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

func service(l *locker, p *publisher) Service {
	return Service{
		Locker:    l,
		Publisher: p,
		NewOpID:   func() string { return "op-1" },
		Now:       func() time.Time { return startedAt },
	}
}

func begun(t *testing.T, h *hold, opts domain.RunOptions) *Op {
	t.Helper()
	op, err := service(&locker{hold: h}, &publisher{}).Begin("create", []domain.Lock{repoLock}, opts)
	if err != nil {
		t.Fatalf("Begin(create) = %v", err)
	}
	return op
}
