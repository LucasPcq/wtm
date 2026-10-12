// Package form is the interactive form of a command on the engine: it fills
// the request from the command's schema, state and choices, shows the plan,
// then runs the plan it showed.
package form

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

type Params struct {
	Entry    dispatch.Fillable
	Request  json.RawMessage
	Options  dispatch.Options
	Names    func(path string) string
	Plan     func(detail json.RawMessage) string
	Progress kernel.Emitter
}

// Run asks what the request lacks, previews it and applies the plan the recap
// showed; facts that changed meanwhile bring the recap back with the new plan.
// Backing out is a kernel.Cancelled. The form draws no conclusion: the reply
// is concluded as a run without a form is.
func Run(ctx context.Context, params Params) (dispatch.Reply, error) {
	styles.UseRendererOn(os.Stderr)
	s := newSession(ctx, params)
	s.program = components.RunWizard
	s.loading = func(params components.LoadingParams) error { return components.RunLoading(ctx, params) }
	return s.run()
}

func (s *session) run() (dispatch.Reply, error) {
	if err := s.observe(); err != nil {
		return nil, err
	}
	if err := s.flag(); err != nil {
		return nil, err
	}
	for {
		confirmed, err := s.fill()
		if err != nil {
			return nil, err
		}
		if s.params.Options.DryRun {
			return s.call(dispatch.Options{DryRun: true})
		}
		if !confirmed {
			return nil, cancelled(nil)
		}
		reply, err := s.call(dispatch.Options{ExpectFingerprint: s.preview.Fingerprint})
		if !isStale(err) {
			return reply, err
		}
		if err := s.observe(); err != nil {
			return nil, err
		}
		s.given, s.stale = s.request, true
	}
}

// call runs the request the form filled, on the TUI's behalf, with the options
// the command line was given.
func (s *session) call(options dispatch.Options) (dispatch.Reply, error) {
	options.IgnoreRemembered = s.params.Options.IgnoreRemembered
	options.CorrelationID = s.params.Options.CorrelationID
	options.Surface = dispatch.SurfaceTUI
	return s.params.Entry.Call(s.ctx, dispatch.Call{Request: s.request, Options: options, Progress: s.params.Progress})
}

func cancelled(cause error) error {
	return kernel.Cancelled(kernel.Problem{Code: kernel.CodeInterrupted, Cause: cause})
}

func isStale(err error) bool {
	var failure kernel.Error
	return errors.As(err, &failure) && failure.Base().Code == kernel.CodePlanStale
}
