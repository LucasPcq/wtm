package domain

type Origin string

const (
	OriginRequest    Origin = "request"
	OriginRemembered Origin = "remembered"
	OriginConfig     Origin = "config"
	OriginDefault    Origin = "default"
)

type Answer[T any] struct {
	Value  T
	Origin Origin
}

type Option struct {
	Value string
	Badge string
}

type Choice[T ~string] struct {
	Answer[T]
	Options    []T
	Skip       string
	Remembered T
}

type Confirm struct {
	Answer[bool]
	Skip string
}
