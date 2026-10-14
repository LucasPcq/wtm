package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// FieldType is the shape of a value, never its subject: a branch picker is a
// select whose choices are searched.
type FieldType string

const (
	FieldText        FieldType = "text"
	FieldBool        FieldType = "bool"
	FieldSelect      FieldType = "select"
	FieldMultiSelect FieldType = "multiselect"
	FieldReorder     FieldType = "reorder"
	FieldTextList    FieldType = "textlist"
	FieldDecisions   FieldType = "decisions"
)

type ChoicesMode string

const (
	ChoicesNone    ChoicesMode = ""
	ChoicesStatic  ChoicesMode = "static"
	ChoicesDynamic ChoicesMode = "dynamic"
	// ChoicesSearch is a long list the user filters, can refresh, with one
	// option pinned on top.
	ChoicesSearch ChoicesMode = "search"
)

// Constraints are checked by CheckSpec and published with the schema; a zero
// bound is no bound.
type Constraints struct {
	Enum     []string `json:"enum,omitempty"`
	MinLen   int      `json:"min_len,omitempty"`
	MaxLen   int      `json:"max_len,omitempty"`
	MinItems int      `json:"min_items,omitempty"`
	MaxItems int      `json:"max_items,omitempty"`
	Pattern  string   `json:"pattern,omitempty"`
}

type FieldSpec struct {
	Path         string      `json:"path"`
	Type         FieldType   `json:"type"`
	Label        string      `json:"label"`
	Title        string      `json:"title,omitempty"`
	Description  string      `json:"description,omitempty"`
	Required     bool        `json:"required,omitempty"`
	DependsOn    []string    `json:"depends_on,omitempty"`
	Choices      ChoicesMode `json:"choices,omitempty"`
	Rememberable bool        `json:"rememberable,omitempty"`
	Constraints  Constraints `json:"constraints"`
}

//go-sumtype:decl Value

// Value is one of Text, Bool, List or Decisions: the Go type of a request
// field decides which, and Set refuses any other. nil is no value.
type Value interface {
	value()
}

// Text is the value of a text or a select field, or of a named string type.
type Text string

type Bool bool

// List is the value of a multiselect, reorder or textlist field.
type List []string

// Decisions is a choice per key.
type Decisions map[string]string

func (Text) value()      {}
func (Bool) value()      {}
func (List) value()      {}
func (Decisions) value() {}

type Origin string

const (
	OriginRequest    Origin = "request"
	OriginRemembered Origin = "remembered"
	OriginConfig     Origin = "config"
	OriginDefault    Origin = "default"
)

type Fallback struct {
	Value  Value
	Origin Origin
}

type FieldStatus string

const (
	FieldMissing   FieldStatus = "missing"
	FieldProvided  FieldStatus = "provided"
	FieldDefaulted FieldStatus = "defaulted"
	FieldSkipped   FieldStatus = "skipped"
)

// FieldState is a field in a form. Remembered is the answer the repository
// remembers for it, kept when the run forgets it (--ask).
type FieldState struct {
	Path       string       `json:"path"`
	Status     FieldStatus  `json:"status"`
	Origin     Origin       `json:"origin,omitempty"`
	Value      Value        `json:"value"`
	Remembered Value        `json:"remembered,omitempty"`
	SkipReason Code         `json:"skip_reason,omitempty"`
	Errors     []FieldError `json:"errors,omitempty"`
}

// UnmarshalJSON reads each Value by its JSON shape: a string is Text, a bool
// Bool, an array List, an object Decisions.
func (s *FieldState) UnmarshalJSON(data []byte) error {
	type plain FieldState
	var decoded struct {
		plain
		Value      json.RawMessage `json:"value"`
		Remembered json.RawMessage `json:"remembered"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	value, err := decodeValue(decoded.Value)
	if err != nil {
		return err
	}
	remembered, err := decodeValue(decoded.Remembered)
	if err != nil {
		return err
	}
	*s = FieldState(decoded.plain)
	s.Value, s.Remembered = value, remembered
	return nil
}

func decodeValue(raw json.RawMessage) (Value, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	switch trimmed[0] {
	case '"':
		return decodeAs[Text](trimmed)
	case 't', 'f':
		return decodeAs[Bool](trimmed)
	case '[':
		return decodeAs[List](trimmed)
	case '{':
		return decodeAs[Decisions](trimmed)
	}
	return nil, fmt.Errorf("kernel: %s is not a value", trimmed)
}

func decodeAs[V Value](raw []byte) (Value, error) {
	var value V
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

type Badge struct {
	Code   Code   `json:"code"`
	Params Params `json:"params,omitempty"`
}

type Option struct {
	Value   string      `json:"value"`
	Label   string      `json:"label"`
	Danger  bool        `json:"danger,omitempty"`
	Refusal *FieldError `json:"refusal,omitempty"`
	Badges  []Badge     `json:"badges,omitempty"`
}

type Choices struct {
	Options []Option `json:"options"`
	Pinned  string   `json:"pinned,omitempty"`
	Banner  *Warning `json:"banner,omitempty"`
}

// FieldDef keeps a field's behaviour in the engine: Skip, Default and Validate
// are pure (no ctx), only Choices may do I/O.
type FieldDef[Req, F any] struct {
	Spec     FieldSpec
	Skip     func(Req, F) (skip bool, reason Code)
	Default  func(Req, F) (Fallback, bool)
	Validate func(Req, F) []FieldError
	Choices  func(context.Context, Req, F) (Choices, error)
}
