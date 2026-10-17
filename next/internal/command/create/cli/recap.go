package cli

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
)

func (w *wizard) previewPlan(ctx context.Context) error {
	w.preview = nil
	outcome, err := w.app.Run(ctx, w.req, operation.Options{DryRun: true, Surface: surfaceTUI})
	if err == nil {
		w.preview = &outcome
	}
	return err
}

func (w *wizard) plan(width int) string {
	return planTextOf(planTextParams{Plan: w.preview.Plan, Warnings: w.preview.Warnings, Width: width})
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
