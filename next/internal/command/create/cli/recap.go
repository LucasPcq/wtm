package cli

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func (w *wizard) previewPlan(ctx context.Context) (func(), error) {
	outcome, err := w.app.Run(ctx, w.req, operation.Options{DryRun: true, Surface: surfaceTUI})
	if err != nil {
		return func() { w.preview = nil }, err
	}
	return func() { w.preview = &outcome }, nil
}

func (w *wizard) plan(width int, st *output.Styles) string {
	return planTextOf(w.preview.Plan, w.preview.Warnings, width, st)
}

func (w *wizard) apply(ctx context.Context) error {
	opts := operation.Options{DryRun: w.dryRun, Surface: surfaceTUI, Report: w.report}
	if !w.dryRun {
		opts.ExpectFingerprint = w.preview.Fingerprint
	}
	outcome, err := w.app.Run(ctx, w.req, opts)
	if !stale(err) {
		w.outcome = outcome
	}
	return err
}

func stale(err error) bool {
	found, ok := errors.AsType[*domain.Error](err)
	return ok && found.Code == operation.CodePlanStale
}
