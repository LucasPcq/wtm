package flow

import "strings"

// SetMemo keeps one value per selection, for a step rebuilt on every move
// through the wizard that would otherwise redo its I/O each time. The zero
// value is ready to use.
type SetMemo[T any] struct {
	values map[string]T
}

func (m *SetMemo[T]) Get(selection []string, load func() T) T {
	key := strings.Join(selection, "\x00")
	if value, cached := m.values[key]; cached {
		return value
	}
	if m.values == nil {
		m.values = map[string]T{}
	}
	value := load()
	m.values[key] = value
	return value
}
