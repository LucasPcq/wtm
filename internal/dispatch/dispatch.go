// Package dispatch is the impure half of the command engine and the one way a
// surface reaches a command: it takes the operation locks, observes, validates,
// plans, checks the fingerprint, applies, and publishes what happened, the same
// way for every command and every surface.
package dispatch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/proc"
	"github.com/LucasPcq/wtm/internal/kernel"
)

const opIDBytes = 8

// Publisher is the event bus; events.Publisher satisfies it.
type Publisher interface {
	Publish(ctx context.Context, event domain.Event)
}

// Params are the ports dispatch itself runs on. A command gets its own ports
// (git, the state files, processes) when it is built, never through dispatch.
type Params struct {
	// Dir is the directory the surface runs from, what a command observes.
	Dir string
	// StateDir is <common>/wtm: the operation locks and the hook logs live
	// there. Empty runs every command unprotected.
	StateDir  string
	Publisher Publisher
	Now       func() time.Time
	NewOpID   func() kernel.OpID
	Shield    kernel.Shield
}

type Dispatcher struct {
	params Params
}

func New(params Params) *Dispatcher {
	if params.Now == nil {
		params.Now = time.Now
	}
	if params.NewOpID == nil {
		params.NewOpID = randomOpID
	}
	if params.Shield == nil {
		params.Shield = proc.Shield
	}
	return &Dispatcher{params: params}
}

func randomOpID() kernel.OpID {
	raw := make([]byte, opIDBytes)
	_, _ = rand.Read(raw)
	return kernel.OpID(hex.EncodeToString(raw))
}

// Options are what a surface asks of one run, beside its request.
type Options struct {
	DryRun bool
	// ExpectFingerprint is the fingerprint a preview returned: the run is
	// refused with plan.stale when the facts it reads have changed since.
	ExpectFingerprint Fingerprint
	// IgnoreRemembered drops the remembered answers before the defaults
	// apply (--ask).
	IgnoreRemembered bool
	Surface          kernel.Surface
	CorrelationID    string
}
