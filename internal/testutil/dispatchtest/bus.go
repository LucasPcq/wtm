package dispatchtest

import (
	"context"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
)

type Bus struct {
	mu     sync.Mutex
	events []domain.Event
}

func (b *Bus) Publish(_ context.Context, event domain.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event)
}

func (b *Bus) Events() []domain.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.events)
}

func (b *Bus) Types() []domain.EventType {
	types := []domain.EventType{}
	for _, event := range b.Events() {
		types = append(types, event.Type)
	}
	return types
}
