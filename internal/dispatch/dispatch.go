// Package dispatch is the impure half of the command engine: the one way a
// surface runs a command.
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

type Publisher interface {
	Publish(ctx context.Context, event domain.Event)
}

// Params are dispatch's own ports; a command gets its ports when it is built.
type Params struct {
	Dir string
	// StateDir is <common>/wtm; empty runs every command unprotected.
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
