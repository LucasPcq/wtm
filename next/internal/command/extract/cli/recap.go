package cli

import (
	"context"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// previewPlan dry-runs the request as answered, for the recap to show the plan its apply will be held to.
func (w *wizard) previewPlan(ctx context.Context) (func(), error) {
	outcome, err := w.app.Run(ctx, w.req, domain.RunOptions{DryRun: true, Surface: uicli.SurfaceTUI})
	if err != nil {
		return func() { w.preview = nil }, err
	}
	return func() { w.preview = &outcome }, nil
}

// plan draws the plan previewed, in width columns.
func (w *wizard) plan(width int, st *output.Styles) string {
	return strings.Join(output.PlanLines(output.PlanParams{Rows: planRowsOf(w.preview.Plan), Warnings: output.Messages(w.preview.Warnings), Width: width, Styles: st}), "\n")
}

// applyLabel names the apply of the plan previewed: it creates the target first when the target does not exist.
func (w *wizard) applyLabel() string {
	switch w.preview.Plan.Target.Action {
	case extract.ActionCreate, extract.ActionCheckOut, extract.ActionResume:
		return "Yes, create & extract"
	case extract.ActionExisting:
	}

	return "Yes, extract"
}

// apply runs the request as answered, held to the fingerprint of the plan previewed unless on a dry run; a stale plan leaves the outcome alone, since the wizard starts again.
func (w *wizard) apply(ctx context.Context) error {
	opts := domain.RunOptions{DryRun: w.dryRun, Surface: uicli.SurfaceTUI, Report: w.report}

	if !w.dryRun {
		opts.ExpectFingerprint = w.preview.Fingerprint
	}

	outcome, err := w.app.Run(ctx, w.req, opts)
	if !domain.PlanStale(err) {
		w.outcome = outcome
	}

	return err
}
