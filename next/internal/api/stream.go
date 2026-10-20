package api

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// A Stream is a command that sends lines until its caller stops it rather than returning one result (wtm events): its name, the schema of its request, and its watch, which sends each line of the call to send until ctx is done.
// It responds once the stream ends: 200 with no error once ctx is done, else the status and envelope of the refusal or failure that ended it, a refusal before any line among them.
type Stream struct {
	Name  string
	Form  domain.Form
	Watch func(ctx context.Context, call Call, send func(line json.RawMessage) error) Response
}

// BindStream returns the stream called name that folds the call's answers into its request, refusing an answer by its path and a relative directory, then watches with watch from the call's directory; the call's options are not read.
func BindStream[Req Request[Req], Q Form](name string, watch func(ctx context.Context, dir string, req Req, send func(line json.RawMessage) error) error) Stream {
	var zero Q

	run := func(ctx context.Context, call Call, send func(json.RawMessage) error) Response {
		// A relative directory would resolve against the server process's own working directory.
		if !filepath.IsAbs(call.Dir) {
			return respond(domain.Envelope{Command: name, Err: domain.InvalidMessage("the directory of a " + name + " call must be absolute, not " + strconv.Quote(call.Dir))})
		}

		var req Req
		req, err := domain.Fold(req, call.Answers)
		if err == nil {
			err = watch(ctx, call.Dir, req, send)
		}
		if err == nil || ctx.Err() != nil {
			return Response{Status: http.StatusOK, Envelope: domain.Envelope{Command: name}}
		}
		return respond(domain.Envelope{Command: name, Err: domain.AsError(domain.AsErrorParams{Err: err})})
	}

	return Stream{Name: name, Form: zero.Form(), Watch: run}
}
