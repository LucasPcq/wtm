package domain

// Origin says where an answer to a question came from.
type Origin string

// The origins of an answer.
const (
	OriginRequest    Origin = "request"    // a flag or the request
	OriginRemembered Origin = "remembered" // the answer remembered in config.toml
	OriginConfig     Origin = "config"     // a value of the project's configuration
	OriginDefault    Origin = "default"    // the question's default
)

// Answer is the value a question settled on, and where that value came from.
type Answer[T any] struct {
	Value  T
	Origin Origin
}

// Option is one value a choice offers, with a badge the wizard shows beside it (`on origin`, a count of changes).
type Option struct {
	Value string
	Badge string
}

// Choice is a question with a closed set of options: its answer, its options, the reason it is not asked (Skip, empty when it is), and the answer remembered for it.
type Choice[T ~string] struct {
	Answer[T]
	Options    []T
	Skip       string
	Remembered T
}

// Confirm is a yes-or-no question: its answer and the reason it is not asked (Skip, empty when it is).
type Confirm struct {
	Answer[bool]
	Skip string
}
