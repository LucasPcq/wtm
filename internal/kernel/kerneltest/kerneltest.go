// Package kerneltest holds the checks every command runs on its own fields and sagas.
package kerneltest

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/kernel"
)

const changed = "kerneltest"

var errInjected = errors.New("kerneltest: injected failure")

type DependsOnParams[Req, F any] struct {
	Fields  []kernel.FieldDef[Req, F]
	Request Req
	Facts   F
}

// CheckDependsOn changes, one at a time, every field a FieldDef does not
// declare in DependsOn, and fails when Skip, Default or Validate notices.
func CheckDependsOn[Req, F any](t testing.TB, params DependsOnParams[Req, F]) {
	t.Helper()
	for _, field := range params.Fields {
		if !resolves[Req](t, field.Spec.Path) {
			continue
		}
		for _, declared := range field.Spec.DependsOn {
			resolves[Req](t, declared)
		}
		baseline := observe(field, params.Request, params.Facts)
		for _, path := range kernel.Paths[Req]() {
			if path == field.Spec.Path || declares(field.Spec.DependsOn, path) {
				continue
			}
			if reads(readsParams[Req, F]{Field: field, Request: params.Request, Facts: params.Facts, Path: path, Baseline: baseline}) {
				t.Errorf("field %q reads %q without declaring it in DependsOn", field.Spec.Path, path)
			}
		}
	}
}

type SagaParams[D any] struct {
	Saga     func() kernel.Saga[D]
	Snapshot func() string
}

// CheckSaga fails each step of the saga in turn, then its commit, and wants
// the state back exactly as it was each time.
func CheckSaga[D any](t testing.TB, params SagaParams[D]) {
	t.Helper()
	steps := len(params.Saga().Steps)
	for point := range steps + 1 {
		before := params.Snapshot()
		saga := failingAt(params.Saga(), point)
		items := kernel.Each(context.Background(), kernel.EachParams[int, D]{
			Items:   []int{point},
			Subject: func(int) string { return failurePoint(saga, point) },
			Unit:    func(int) kernel.Unit[D] { return kernel.Unit[D]{Saga: saga} },
		})
		if items[0].Status != kernel.StatusFailed {
			t.Errorf("saga failing at %s: status %s, want failed", failurePoint(saga, point), items[0].Status)
		}
		if after := params.Snapshot(); after != before {
			t.Errorf("saga failing at %s left the state changed:\nbefore: %s\nafter:  %s", failurePoint(saga, point), before, after)
		}
	}
}

type observation struct {
	Skip     bool
	Reason   kernel.Code
	Fallback kernel.Fallback
	Defaults bool
	Problems []kernel.FieldError
}

func observe[Req, F any](field kernel.FieldDef[Req, F], req Req, facts F) observation {
	var seen observation
	if field.Skip != nil {
		seen.Skip, seen.Reason = field.Skip(req, facts)
	}
	if field.Default != nil {
		seen.Fallback, seen.Defaults = field.Default(req, facts)
	}
	if field.Validate != nil {
		seen.Problems = field.Validate(req, facts)
	}
	return seen
}

// resolves accepts a leaf field or a nested struct holding some.
func resolves[Req any](t testing.TB, path string) bool {
	t.Helper()
	known := slices.ContainsFunc(kernel.Paths[Req](), func(leaf string) bool {
		return kernel.Within(kernel.PathIn{Path: leaf, Parent: path})
	})
	if !known {
		var req Req
		t.Errorf("path %q names no field of %T", path, req)
	}
	return known
}

type readsParams[Req, F any] struct {
	Field    kernel.FieldDef[Req, F]
	Request  Req
	Facts    F
	Path     string
	Baseline observation
}

// reads tries the field at Path emptied, then changed: either one is enough
// to show the FieldDef looks at it.
func reads[Req, F any](params readsParams[Req, F]) bool {
	value, err := kernel.Get(params.Request, params.Path)
	if err != nil {
		return false
	}
	for _, other := range []kernel.Value{nil, changedFrom(value)} {
		mutated, err := kernel.Set(params.Request, params.Path, other)
		if err != nil {
			return false
		}
		if !reflect.DeepEqual(observe(params.Field, mutated, params.Facts), params.Baseline) {
			return true
		}
	}
	return false
}

func declares(dependsOn []string, path string) bool {
	return slices.ContainsFunc(dependsOn, func(declared string) bool {
		return kernel.Within(kernel.PathIn{Path: path, Parent: declared})
	})
}

// changedFrom returns a value of the same shape that differs from value.
func changedFrom(value kernel.Value) kernel.Value {
	switch v := value.(type) {
	case kernel.Text:
		return v + changed
	case kernel.Bool:
		return !v
	case kernel.List:
		return append(slices.Clone(v), changed)
	case kernel.Decisions:
		decisions := maps.Clone(v)
		if decisions == nil {
			decisions = kernel.Decisions{}
		}
		decisions[changed] = changed
		return decisions
	}
	return value
}

func failingAt[D any](saga kernel.Saga[D], point int) kernel.Saga[D] {
	steps := slices.Clone(saga.Steps)
	if point < len(steps) {
		steps[point].Do = func(context.Context) error { return errInjected }
		return kernel.Saga[D]{Steps: steps, Commit: saga.Commit, LeftBehind: saga.LeftBehind}
	}
	commit := func(context.Context) (D, error) {
		var none D
		return none, errInjected
	}
	return kernel.Saga[D]{Steps: steps, Commit: commit, LeftBehind: saga.LeftBehind}
}

func failurePoint[D any](saga kernel.Saga[D], point int) string {
	if point < len(saga.Steps) {
		return "step " + saga.Steps[point].Name
	}
	return "commit"
}
