// Package operation is what every mutating use case goes through: its record, its locks, its plan under them and fingerprinted, its events.
package operation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"os"
	"slices"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	CodePlanStale    = "plan.stale"
	fingerprintBytes = 12
)

type Service struct {
	Locker        Locker
	Publisher     Publisher
	NewOpID       func() string
	Now           func() time.Time
	CorrelationID string
}

type Options struct {
	DryRun            bool
	ExpectFingerprint string
	Surface           string
	Report            domain.Report
}

type BeginParams struct {
	Command string
	Locks   []domain.Lock
	Options Options
}

type Op struct {
	ID      string
	record  domain.Operation
	service Service
	options Options
	hold    Hold
	report  domain.Report
}

func (s Service) Begin(params BeginParams) (*Op, error) {
	op := &Op{
		record: domain.Operation{
			ID:            s.NewOpID(),
			PID:           os.Getpid(),
			Command:       params.Command,
			Surface:       params.Options.Surface,
			CorrelationID: s.CorrelationID,
			StartedAt:     s.Now().UTC(),
			Locks:         params.Locks,
		},
		service: s,
		options: params.Options,
	}
	op.ID = op.record.ID
	if params.Options.DryRun {
		return op, nil
	}
	hold, err := s.Locker.Take(op.record)
	if err != nil {
		return op, err
	}
	op.hold = hold
	return op, nil
}

func PlanUnderLocks[P interface{ Locks() []domain.Lock }](ctx context.Context, op *Op, planOf func(ctx context.Context) (P, []domain.Warning, error)) (P, []domain.Warning, error) {
	for {
		planned, warnings, err := planOf(ctx)
		if err != nil {
			return planned, nil, err
		}
		if op.hold == nil {
			return planned, warnings, nil
		}
		grown, err := op.hold.Grow(planned.Locks())
		if err != nil || !grown {
			return planned, warnings, err
		}
		// A lock the defaults named and the request did not is taken first, then the facts are observed again: what it guards may have changed meanwhile.
	}
}

func Fingerprint(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:fingerprintBytes]), nil
}

func (o *Op) Expect(fingerprint string) error {
	expected := o.options.ExpectFingerprint
	if expected == "" || expected == fingerprint {
		return nil
	}
	return &domain.Error{
		Kind:    domain.KindPrecondition,
		Code:    CodePlanStale,
		Message: "the facts changed since the preview",
		Params:  domain.Params{"expected": expected, "found": fingerprint},
	}
}

func (o *Op) Started() domain.Report {
	publish := o.service.Publisher.Open(o.record)
	surface := o.options.Report
	o.report = func(event domain.Event) {
		publish(event)
		if surface != nil {
			surface(event)
		}
	}
	o.report(domain.OperationStarted{Command: o.record.Command, Surface: o.record.Surface})
	return o.report
}

func (o *Op) Finished(failure *domain.Error) {
	finished := domain.OperationFinished{Command: o.record.Command, Surface: o.record.Surface, OK: true}
	if failure != nil {
		finished.OK, finished.Code = false, failure.Code
	}
	o.report(finished)
}

func (o *Op) Warnings(warnings []domain.Warning) []domain.Warning {
	if o.hold == nil {
		return warnings
	}
	if !o.hold.Protected() {
		warnings = append(warnings, domain.LocksUnavailable)
	}
	return once(warnings)
}

func (o *Op) End() {
	if o.hold != nil {
		o.hold.Release()
	}
}

func once(warnings []domain.Warning) []domain.Warning {
	var kept []domain.Warning
	for _, warning := range warnings {
		seen := slices.ContainsFunc(kept, func(k domain.Warning) bool {
			return k.Code == warning.Code && k.Message == warning.Message && maps.Equal(k.Params, warning.Params)
		})
		if !seen {
			kept = append(kept, warning)
		}
	}
	return kept
}
