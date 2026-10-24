package formtest

import (
	"context"
	"fmt"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// Previewing stands in for a command's use case: each preview runs the use case it wraps and gets a fingerprint of its own (preview-1, preview-2…), and each apply ends on the next of Errs. It records in Held the fingerprint each apply was held to, and in IDs the correlation ID of each run, previews included.
type Previewing[R, Q, P, D any] struct {
	form.UseCase[R, Q, domain.Outcome[P, D]]
	Errs []error
	Held []string
	IDs  []string

	previews int
}

// Run previews req through the use case it wraps, or ends the apply on the next of Errs.
func (p *Previewing[R, Q, P, D]) Run(ctx context.Context, req R, opts domain.RunOptions) (domain.Outcome[P, D], error) {
	p.IDs = append(p.IDs, opts.CorrelationID)
	if opts.DryRun {
		p.previews++
		outcome, err := p.UseCase.Run(ctx, req, opts)
		outcome.Fingerprint = fmt.Sprintf("preview-%d", p.previews)
		return outcome, err
	}

	p.Held = append(p.Held, opts.ExpectFingerprint)
	err := p.Errs[0]
	p.Errs = p.Errs[1:]
	return domain.Outcome[P, D]{}, err
}
