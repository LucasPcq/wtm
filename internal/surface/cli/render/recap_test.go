package render_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

func TestARecapIsThePlanThenItsFingerprintThenHowToApplyIt(t *testing.T) {
	var out bytes.Buffer

	render.Recap(&out, render.RecapParams{Catalog: catalog,
		Command:     "make",
		Plan:        json.RawMessage(`{"branches":["feat/a"]}`),
		Warnings:    []kernel.Warning{{Code: kernel.CodeLocksUnavailable}},
		Fingerprint: "abc",
		Next:        []render.NextStepParams{{Command: "wtm make feat/a --expect-fingerprint abc", Note: "apply this plan"}},
		Detail:      render.GenericDetail{},
	})

	assert.Equal(t, lines(
		"  make · dry run, nothing applied",
		"  Branches  feat/a",
		"",
		"  ! ran without operation locks",
		"",
		"  fingerprint  abc",
		"  → wtm make feat/a --expect-fingerprint abc   apply this plan",
	), plain(out.String()))
}

func TestARecapOfAPlanThatDoesNothingOffersNothingToApply(t *testing.T) {
	var out bytes.Buffer

	render.Recap(&out, render.RecapParams{Catalog: catalog,
		Command:     "make",
		Plan:        json.RawMessage(`{}`),
		Fingerprint: "abc",
		Next:        []render.NextStepParams{{Command: "wtm make --expect-fingerprint abc", Note: "apply this plan"}},
		Detail:      render.GenericDetail{},
	})

	assert.Equal(t, lines(
		"  make · dry run, nothing applied",
		"  = nothing to do",
		"  fingerprint  abc",
	), plain(out.String()))
}
