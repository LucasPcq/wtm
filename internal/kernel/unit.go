package kernel

import "context"

type Shield func(context.Context) (context.Context, func())

type Prep struct {
	Name string
	Log  string
	Run  func(context.Context, Reporter) error
}

type SagaStep struct {
	Name string
	Do   func(context.Context, Reporter) error
	Undo func(context.Context) error
}

type Saga[D any] struct {
	Steps      []SagaStep
	Commit     func(context.Context) (D, error)
	LeftBehind func(step string) *FollowUp
}

type Phase[D any] struct {
	Name string
	Log  string
	Run  func(context.Context, Reporter, D) (D, error)
}

type Unit[D any] struct {
	Before []Prep
	Saga   Saga[D]
	Then   []Phase[D]
}
