package dispatchtest

import (
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/internal/kernel"
)

type Progress struct {
	mu   sync.Mutex
	seen []kernel.Progress
}

func (p *Progress) Emit(progress kernel.Progress) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = append(p.seen, progress)
}

func (p *Progress) Seen() []kernel.Progress {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.seen)
}
