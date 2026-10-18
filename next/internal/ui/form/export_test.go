package form

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/spf13/cobra"
)

func (c *Command[R, Q, O]) SetRun(run *uicli.Run, yes bool, expect string) {
	c.io, c.yes, c.expect = run, yes, expect
}

func (c *Command[R, Q, O]) Interactive() bool {
	return c.interactive()
}

func (c *Command[R, Q, O]) RequestOf(cmd *cobra.Command, args []string) R {
	return c.request(cmd, args)
}

func (s *Session[R, Q, O]) Remembering(at int, path string, on bool, answers ...domain.Answered) {
	s.remembering(at, path, on, answers...)
}

func (s *Session[R, Q, O]) Observe(ctx context.Context) (func(), error) {
	return s.observe(ctx)
}

func (s *Session[R, Q, O]) Apply(ctx context.Context) error {
	return s.apply(ctx)
}

func (s *Session[R, Q, O]) Outcome() O {
	return s.outcome
}
