package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// The codes of the server's own refusals, before any command runs.
const (
	CodeTokenInvalid      = "web.token_invalid"
	CodeHostRefused       = "web.host_refused"
	CodeOriginRefused     = "web.origin_refused"
	CodeNotFound          = "web.not_found"
	CodeRepoUnknown       = "web.repo_unknown"
	CodeCommandUnknown    = "web.command_unknown"
	CodeOperationUnknown  = "web.operation_unknown"
	CodeSchemaUnavailable = "web.schema_unavailable"
)

// statusClientClosed is the status of an operation its caller cancelled: nginx's 499, which net/http does not name.
const statusClientClosed = 499

var statusByKind = map[domain.Kind]int{
	domain.KindInvalid:      http.StatusBadRequest,
	domain.KindRefused:      http.StatusForbidden,
	domain.KindConflict:     http.StatusConflict,
	domain.KindPrecondition: http.StatusPreconditionFailed,
	domain.KindCancelled:    statusClientClosed,
	domain.KindInternal:     http.StatusInternalServerError,
}

// statusOf is the HTTP status of how a command ended: 200 when it succeeded, else the status of its error's kind, 500 for a kind it does not know.
func statusOf(envelope domain.Envelope) int {
	if envelope.Err == nil {
		return http.StatusOK
	}

	status, known := statusByKind[envelope.Err.Kind]
	if !known {
		return http.StatusInternalServerError
	}
	return status
}

func writeEnvelope(w http.ResponseWriter, envelope domain.Envelope) {
	writeJSON(w, statusOf(envelope), envelope)
}

// writeError writes the envelope of a refusal of the server itself, with status.
func writeError(w http.ResponseWriter, status int, err *domain.Error) {
	writeJSON(w, status, domain.Envelope{Err: err})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		status, data = http.StatusInternalServerError, []byte(`{"error":"the response could not be encoded"}`)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

// maxBody bounds the body of a request: a call's answers are a few kilobytes.
const maxBody = 1 << 20

// decode reads r's body as JSON into v, an empty body leaving v as it is, and refuses one past maxBody.
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func acceptsEvents(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}
