package domain

import (
	"encoding/json"
	"fmt"
)

// StreamVersion is the version of the event stream's lines this wtm writes and reads: a line of a greater version is a newer wtm's, which this one cannot read.
const StreamVersion = 2

// A Repo is a repository as the event stream names it: its main worktree, Root, and git's common directory, CommonDir, its key, the same from any of its worktrees.
type Repo struct {
	Root      string `json:"root"`
	CommonDir string `json:"common_dir"`
}

// A Line is one line of the event stream: its Event, and what the stream stamps on it: the version V, the time TS (RFC 3339, UTC), the repository it is about (none for [Ready]), and the correlation ID of the caller and the operation it comes from, if any.
type Line struct {
	V             int
	TS            string
	Repo          *Repo
	CorrelationID string
	OpID          string
	Event         Event
}

// head is a line's envelope, its fields first on the wire, the event's after.
type head struct {
	V             int    `json:"v"`
	Type          string `json:"type"`
	TS            string `json:"ts"`
	Repo          *Repo  `json:"repo,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	OpID          string `json:"op_id,omitempty"`
}

// MarshalLine returns line as one JSON object: its envelope, the event's type among it, followed by the event's fields.
func MarshalLine(line Line) ([]byte, error) {
	envelope, err := encoded(head{V: line.V, Type: EventType(line.Event), TS: line.TS, Repo: line.Repo, CorrelationID: line.CorrelationID, OpID: line.OpID})
	if err != nil {
		return nil, err
	}

	body, err := encoded(line.Event)
	if err != nil {
		return nil, err
	}
	if len(body) < 2 || body[0] != '{' {
		return nil, fmt.Errorf("event %q: expected a JSON object, got %s", EventType(line.Event), body)
	}
	if string(body) == "{}" {
		return envelope, nil
	}
	return append(append(envelope[:len(envelope)-1], ','), body[1:]...), nil
}

// UnmarshalLine returns the line data encodes, as [MarshalLine] writes it. Of a line whose type it does not know it returns the envelope, no event and an error wrapping [ErrUnknownEventType]; a line of a version greater than [StreamVersion] it decodes no further than its envelope, the caller refusing it by V.
func UnmarshalLine(data []byte) (Line, error) {
	var h head
	if err := json.Unmarshal(data, &h); err != nil {
		return Line{}, err
	}

	line := Line{V: h.V, TS: h.TS, Repo: h.Repo, CorrelationID: h.CorrelationID, OpID: h.OpID}
	if h.V > StreamVersion {
		return line, nil
	}

	event, err := UnmarshalEvent(data)
	line.Event = event
	return line, err
}
