// Package operation is what every mutating use case goes through: its record, its locks, its plan under them and fingerprinted, its events.
package operation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"slices"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	// CodePlanStale is the code of the refusal Op.Expect returns when the plan
	// no longer matches the fingerprint of its preview.
	CodePlanStale = "plan.stale"

	// fingerprintBytes is how many bytes of the plan's SHA-256 a fingerprint keeps.
	fingerprintBytes = 12
)

// Service begins the operations of the mutating use cases. Its fields are the
// adapters and clocks it records an operation with; the zero value is not usable.
type Service struct {
	Locker        Locker
	Publisher     Publisher
	NewOpID       func() string
	Now           func() time.Time
	CorrelationID string
}

// Options are how a surface runs an operation.
type Options struct {
	DryRun            bool          // plan without taking locks, stop after the fingerprint
	ExpectFingerprint string        // refuse to apply a plan whose fingerprint differs; empty accepts any
	Surface           string        // the surface that started the operation (cli, ui…), recorded in its events
	Report            domain.Report // receives each event as it is published; may be nil
}

// An Op is one running operation: its record, the locks it holds and where its
// events go. It is used by one goroutine, from Begin to End.
type Op struct {
	ID      string
	record  domain.Operation
	service Service
	options Options
	hold    Hold
	report  domain.Report
}

// Begin records an operation of command and takes locks, unless opts is a dry
// run, which takes none. When another operation holds one of the locks it
// returns the Op, whose ID still names the attempt, with the Locker's error.
// The caller ends the returned Op with End.
func (s Service) Begin(command string, locks []domain.Lock, opts Options) (*Op, error) {
	op := &Op{
		record: domain.Operation{
			ID:            s.NewOpID(),
			PID:           os.Getpid(),
			Command:       command,
			Surface:       opts.Surface,
			CorrelationID: s.CorrelationID,
			StartedAt:     s.Now().UTC(),
			Locks:         locks,
		},
		service: s,
		options: opts,
	}
	op.ID = op.record.ID

	if opts.DryRun {
		return op, nil
	}

	hold, err := s.Locker.Take(op.record)
	if err != nil {
		return op, err
	}

	op.hold = hold
	return op, nil
}

// PlanUnderLocks plans with planOf and returns its plan and warnings once op
// holds every lock the plan names. A plan naming a lock op did not hold yet is
// planned again after that lock is taken; a dry run plans once. It returns
// planOf's error, or the error of taking the new locks.
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

// Fingerprint returns a short hex digest of v's JSON encoding, which changes
// whenever what would be applied changes. It fails only when v cannot be
// encoded.
func Fingerprint(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:fingerprintBytes]), nil
}

// Expect refuses a plan whose fingerprint differs from the one the options
// expect, with a precondition error coded CodePlanStale. With no expected
// fingerprint it accepts any plan.
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

// Stale reports whether err is the refusal of a plan the facts changed under.
func Stale(err error) bool {
	found, ok := errors.AsType[*domain.Error](err)
	return ok && found.Code == CodePlanStale
}

// Started publishes the operation's started event and returns the Report that
// sends each later event both to the other processes and to the surface.
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

// Finished publishes the operation's finished event, failed with failure's code
// when failure is not nil. It must follow Started.
func (o *Op) Finished(failure *domain.Error) {
	finished := domain.OperationFinished{Command: o.record.Command, Surface: o.record.Surface, OK: true}
	if failure != nil {
		finished.OK, finished.Code = false, failure.Code
	}

	o.report(finished)
}

// Warnings returns warnings with each one kept once, plus
// domain.LocksUnavailable when the operation ran without its locks.
func (o *Op) Warnings(warnings []domain.Warning) []domain.Warning {
	if o.hold == nil {
		return warnings
	}

	if !o.hold.Protected() {
		warnings = append(warnings, domain.LocksUnavailable)
	}
	return once(warnings)
}

// End releases the operation's locks; it is safe on an Op that holds none.
func (o *Op) End() {
	if o.hold != nil {
		o.hold.Release()
	}
}

// once returns warnings without the repeats of a warning already kept, in order.
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
