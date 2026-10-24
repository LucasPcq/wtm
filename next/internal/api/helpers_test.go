package api

import (
	"context"
	"encoding/json"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// greet is the command of the catalogue over useCase, opened by open.
func greet(open func(ctx context.Context, dir string) (useCase, error)) Command {
	return Bind("greet", open, useCase.Questions, useCase.Run)
}

// named is the answer of the request that names name.
func named(name string) []domain.Answered {
	return []domain.Answered{{Path: "name", Values: []string{name}}}
}

// sink drops every line.
func sink() Sink {
	return Sink{Send: func(json.RawMessage) error { return nil }}
}
