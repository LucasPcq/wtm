package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/require"
)

// useCase is the stream over bus, registry and the repositories open opens, its clock fixed and its checks every recheck.
func useCase(bus *fakeBus, registry *fakeRegistry, open events.Open) events.UseCase {
	return events.UseCase{Bus: bus, Registry: registry, Open: open, Now: func() time.Time { return fixedNow }, Recheck: time.Hour}
}

// watching runs u.Watch with req in the background until the test ends, and returns the reader it streams to and the channel its error arrives on.
func watching(t *testing.T, u events.UseCase, req events.Request) (*reader, <-chan error) {
	t.Helper()
	r := newReader()
	return r, watchingWith(t, watchingParams{UseCase: u, Request: req, Reader: r})
}

type watchingParams struct {
	UseCase events.UseCase
	Request events.Request
	Reader  *reader
	Context context.Context
}

func watchingWith(t *testing.T, params watchingParams) <-chan error {
	t.Helper()
	ctx := params.Context
	if ctx == nil {
		ctx = t.Context()
	}
	done := make(chan error, 1)
	go func() { done <- params.UseCase.Watch(ctx, params.Request, params.Reader.out()) }()
	return done
}

// next returns the next line r received, as JSON, failing after two seconds without one.
func (r *reader) next(t *testing.T) string {
	t.Helper()
	select {
	case received := <-r.lines:
		return string(received.JSON)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "the stream sent nothing")
		return ""
	}
}

// nextType returns the type of the next line r received.
func (r *reader) nextType(t *testing.T) string {
	t.Helper()
	var head struct {
		Type string `json:"type"`
	}
	require.NoError(t, json.Unmarshal([]byte(r.next(t)), &head))
	return head.Type
}

// quiet fails the test when r receives a line within a short while.
func (r *reader) quiet(t *testing.T) {
	t.Helper()
	select {
	case received := <-r.lines:
		require.FailNow(t, "the stream sent "+string(received.JSON))
	case <-time.After(150 * time.Millisecond):
	}
}

// warning returns the next warning r received.
func (r *reader) warning(t *testing.T) error {
	t.Helper()
	select {
	case err := <-r.warnings:
		return err
	case <-time.After(2 * time.Second):
		require.FailNow(t, "no warning")
		return nil
	}
}

// subscribed returns the next subscription made on b.
func (b *fakeBus) subscribed(t *testing.T) subscription {
	t.Helper()
	select {
	case sub := <-b.subscriptions:
		return sub
	case <-time.After(2 * time.Second):
		require.FailNow(t, "nothing subscribed")
		return subscription{}
	}
}

// ended returns the error done receives, failing after two seconds without one.
func ended(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		require.FailNow(t, "the stream did not end")
		return nil
	}
}

// published is the line a publisher of repo writes for event.
func published(t *testing.T, repo domain.Repo, event domain.Event) []byte {
	t.Helper()
	data, err := domain.MarshalLine(domain.Line{V: domain.StreamVersion, TS: "2026-10-16T12:00:00Z", Repo: &repo, OpID: "op-1", Event: event})
	require.NoError(t, err)
	return data
}

// codeOf is the code of err, a *domain.Error.
func codeOf(t *testing.T, err error) string {
	t.Helper()
	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	return failure.Code
}
