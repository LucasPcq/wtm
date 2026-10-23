package domain

// EnvelopeVersion is the version of the envelope's JSON document, its "v".
const EnvelopeVersion = 2

// Envelope is how a command's run ended, as every surface reports it: its operation, its items once applied, the plan and its fingerprint on a dry run, the commands to run next, its warnings and its failure.
type Envelope struct {
	Command     string
	Op          string // the operation's id
	Items       []Item
	Plan        any // the plan a dry run shows, encoded as it is
	Fingerprint string
	FollowUps   []FollowUp
	Warnings    []Warning
	Err         *Error // nil when the command succeeded
}

// An Item is the outcome of one subject of an operation, a worktree for instance.
type Item struct {
	Subject  string
	Status   Status
	Reason   Reason
	Err      *Error
	Warnings []Warning
	Detail   any // what the command reports of the subject, encoded as {} when nil
}

// DetailOf is detail as an Item's Detail: nil when detail is, rather than a nil pointer in an interface, encoded null instead of {}.
func DetailOf[T any](detail *T) any {
	if detail == nil {
		return nil
	}
	return detail
}

// Spell spells the path of a request's field as a surface names it: a flag on the command line. A nil Spell leaves the path as it is.
type Spell func(path string) string

// JSON returns the envelope's v2 document, with follow_ups and warnings always arrays and each field an error refuses spelled by spell; its field names and their presence are a contract scripts rely on.
func (e Envelope) JSON(spell Spell) ([]byte, error) {
	doc := envelopeDoc{V: EnvelopeVersion, Command: e.Command, OK: e.Err == nil, Op: e.Op, Plan: e.Plan, Fingerprint: e.Fingerprint, FollowUps: e.FollowUps, Warnings: e.Warnings, Error: errorDocOf(e.Err, spell)}
	for _, item := range e.Items {
		doc.Items = append(doc.Items, itemDocOf(item, spell))
	}
	if doc.FollowUps == nil {
		doc.FollowUps = []FollowUp{}
	}
	if doc.Warnings == nil {
		doc.Warnings = []Warning{}
	}

	return encoded(doc)
}

// MarshalJSON is the envelope's document with each field named by its path, as a surface that has no flags reports it.
func (e Envelope) MarshalJSON() ([]byte, error) {
	return e.JSON(nil)
}

// MarshalJSON is the error as the envelope's "error" writes it, each field named by its path.
func (e *Error) MarshalJSON() ([]byte, error) {
	return encoded(errorDocOf(e, nil))
}

// MarshalJSON is the field error as the envelope's "fields" write it, named by its path.
func (f FieldError) MarshalJSON() ([]byte, error) {
	return encoded(fieldDocOf(f, nil))
}

type envelopeDoc struct {
	V           int        `json:"v"`
	Command     string     `json:"command"`
	OK          bool       `json:"ok"`
	Op          string     `json:"op,omitempty"`
	Items       []itemDoc  `json:"items,omitempty"`
	Plan        any        `json:"plan,omitempty"`
	Fingerprint string     `json:"fingerprint,omitempty"`
	FollowUps   []FollowUp `json:"follow_ups"`
	Warnings    []Warning  `json:"warnings"`
	Error       *errorDoc  `json:"error,omitempty"`
}

type itemDoc struct {
	Subject  string    `json:"subject"`
	Status   Status    `json:"status"`
	Reason   Reason    `json:"reason,omitempty"`
	Error    *errorDoc `json:"error,omitempty"`
	Warnings []Warning `json:"warnings,omitempty"`
	Detail   any       `json:"detail"`
}

type errorDoc struct {
	Kind     Kind       `json:"kind"`
	Code     string     `json:"code"`
	Message  string     `json:"message"`
	Params   Params     `json:"params,omitempty"`
	FollowUp *FollowUp  `json:"follow_up,omitempty"`
	Fields   []fieldDoc `json:"fields,omitempty"`
}

type fieldDoc struct {
	Path     string    `json:"path"`
	Code     string    `json:"code"`
	Params   Params    `json:"params,omitempty"`
	Accepted []string  `json:"accepted,omitempty"`
	With     []string  `json:"with,omitempty"`
	Flag     string    `json:"flag"`
	Message  string    `json:"message"`
	FollowUp *FollowUp `json:"follow_up,omitempty"`
}

func itemDocOf(item Item, spell Spell) itemDoc {
	detail := item.Detail
	if detail == nil {
		detail = struct{}{}
	}

	return itemDoc{Subject: item.Subject, Status: item.Status, Reason: item.Reason, Error: errorDocOf(item.Err, spell), Warnings: item.Warnings, Detail: detail}
}

func errorDocOf(err *Error, spell Spell) *errorDoc {
	if err == nil {
		return nil
	}

	doc := &errorDoc{Kind: err.Kind, Code: err.Code, Message: err.Text(), Params: err.Params, FollowUp: err.FollowUp}
	for _, field := range err.Fields {
		doc.Fields = append(doc.Fields, fieldDocOf(field, spell))
	}

	return doc
}

func fieldDocOf(field FieldError, spell Spell) fieldDoc {
	flag := field.Path
	if spell != nil {
		flag = spell(field.Path)
	}

	return fieldDoc{Path: field.Path, Code: field.Code, Params: field.Params, Accepted: field.Accepted, With: field.With, Flag: flag, Message: field.Message(flag), FollowUp: field.FollowUp}
}
