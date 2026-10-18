package formtest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// An Envelope is the v2 envelope a command writes, its plan decoded as P.
type Envelope[P any] struct {
	V           int    `json:"v"`
	Command     string `json:"command"`
	OK          bool   `json:"ok"`
	Op          string `json:"op"`
	Items       []Item `json:"items"`
	Plan        *P     `json:"plan"`
	Fingerprint string `json:"fingerprint"`
	Error       *Error `json:"error"`
}

// An Item is an item of an [Envelope], its detail left raw.
type Item struct {
	Subject string          `json:"subject"`
	Status  string          `json:"status"`
	Reason  string          `json:"reason"`
	Detail  json.RawMessage `json:"detail"`
}

// An Error is the error of an [Envelope].
type Error struct {
	Kind    string       `json:"kind"`
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields"`
}

// A FieldError is a field an [Error] refuses.
type FieldError struct {
	Path string `json:"path"`
	Code string `json:"code"`
	Flag string `json:"flag"`
}

// Decoded is the envelope stdout holds.
func Decoded[P any](t *testing.T, stdout string) Envelope[P] {
	t.Helper()
	var doc Envelope[P]
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc), stdout)
	return doc
}
