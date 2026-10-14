package dispatch

import (
	"context"
	"slices"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
)

type Entry interface {
	Name() string
	Schema() []kernel.FieldSpec
	Messages() text.Messages
	ExitCodes() map[kernel.Code]int
	Call(ctx context.Context, call Call) (Reply, error)
	Fillable() (Fillable, bool)
}

type Registry []Entry

// Catalog panics on a code two entries declare.
func (r Registry) Catalog() text.Catalog {
	sets := make([]text.Messages, 0, len(r))
	for _, entry := range r {
		sets = append(sets, entry.Messages())
	}
	return text.NewCatalog(sets...)
}

func (r Registry) Lookup(name string) (Entry, bool) {
	index := slices.IndexFunc(r, func(entry Entry) bool { return entry.Name() == name })
	if index < 0 {
		return nil, false
	}
	return r[index], true
}
