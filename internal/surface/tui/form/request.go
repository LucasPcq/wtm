package form

import (
	"encoding/json"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

func (s *session) requestFrom(prev []components.Step) json.RawMessage {
	request := s.given
	var remembered []string
	offered := false
	for i, step := range prev {
		if i >= len(s.fields) || !s.decided[i].asked {
			continue
		}
		spec := s.fields[i]
		request = s.set(request, spec.Path, valueOf(step.Model))
		list, isList := step.Model.(components.SelectListModel)
		if !isList || !spec.Rememberable {
			continue
		}
		offered = true
		if list.Remembering() {
			remembered = append(remembered, spec.Path)
		}
	}
	if s.remember && offered {
		request = s.set(request, kernel.RememberPath, kernel.List(remembered))
	}
	return request
}

func (s *session) set(request json.RawMessage, path string, value kernel.Value) json.RawMessage {
	updated, err := s.params.Entry.Set(dispatch.SetParams{Request: request, Path: path, Value: value})
	if err != nil {
		s.fail(err)
		return request
	}
	return updated
}
