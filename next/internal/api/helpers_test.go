package api

import "context"

// greet is the command of the catalogue over useCase, opened by open.
func greet(open func(ctx context.Context, dir string) (useCase, error)) Command {
	return Bind("greet", open, useCase.Questions, useCase.Run)
}
