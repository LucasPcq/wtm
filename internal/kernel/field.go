package kernel

import "context"

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
	ChoicesSearch  ChoicesMode = "search"
)

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

type FieldDef[Req, F any] struct {
	Spec     FieldSpec
	Skip     func(Req, F) (skip bool, reason Code)
	Default  func(Req, F) (Fallback, bool)
	Validate func(Req, F) []FieldError
	Choices  func(context.Context, Req, F) (Choices, error)
}
