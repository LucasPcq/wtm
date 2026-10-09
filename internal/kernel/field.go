package kernel

import "context"

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

// Value is a field's value whatever its type: text and select in Text, bool in
// Bool, lists in List, decisions in Decisions.
type Value struct {
	Text      string            `json:"text,omitempty"`
	Bool      bool              `json:"bool,omitempty"`
	List      []string          `json:"list,omitempty"`
	Decisions map[string]string `json:"decisions,omitempty"`
}

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

type FieldState struct {
	Path       string       `json:"path"`
	Status     FieldStatus  `json:"status"`
	Origin     Origin       `json:"origin,omitempty"`
	Value      Value        `json:"value"`
	SkipReason Code         `json:"skip_reason,omitempty"`
	Errors     []FieldError `json:"errors,omitempty"`
}

type Badge struct {
	Code   Code              `json:"code"`
	Params map[string]string `json:"params,omitempty"`
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
