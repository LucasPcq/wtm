package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/service/operation"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// previewPlan dry-runs the request as answered, for the recap to show the plan its apply will be held to.
func (w *wizard) previewPlan(ctx context.Context) (func(), error) {
	outcome, err := w.app.Run(ctx, w.req, operation.Options{DryRun: true, Surface: uicli.SurfaceTUI})
	if err != nil {
		return func() { w.preview = nil }, err
	}
	return func() { w.preview = &outcome }, nil
}

// plan draws the plan previewed, in width columns.
func (w *wizard) plan(width int, st *output.Styles) string {
	return planTextOf(w.preview.Plan, w.preview.Warnings, width, st)
}

// apply runs the request as answered, held to the fingerprint of the plan previewed unless on a dry run; a stale plan leaves the outcome alone, since the wizard starts again.
func (w *wizard) apply(ctx context.Context) error {
	opts := operation.Options{DryRun: w.dryRun, Surface: uicli.SurfaceTUI, Report: w.report}

	if !w.dryRun {
		opts.ExpectFingerprint = w.preview.Fingerprint
	}

	outcome, err := w.app.Run(ctx, w.req, opts)
	if !operation.Stale(err) {
		w.outcome = outcome
	}

	return err
}
