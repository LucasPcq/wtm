package kerneltest_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/kernel"
)

// spy stands in for *testing.T to see what a check reports.
type spy struct {
	testing.TB
	failures []string
}

func (s *spy) Helper() {}

func (s *spy) Errorf(format string, args ...any) {
	s.failures = append(s.failures, fmt.Sprintf(format, args...))
}

type options struct {
	Mode string `json:"mode"`
}

type request struct {
	Branches []string          `json:"branches"`
	From     string            `json:"from"`
	Force    bool              `json:"force"`
	Env      map[string]string `json:"env"`
	Options  options           `json:"options"`
}

type facts struct{ Base string }

func fromField(dependsOn ...string) kernel.FieldDef[request, facts] {
	return kernel.FieldDef[request, facts]{
		Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect, DependsOn: dependsOn},
		Skip: func(req request, _ facts) (bool, kernel.Code) { return req.Options.Mode == "detached", "test.detached" },
		Default: func(req request, f facts) (kernel.Fallback, bool) {
			return kernel.Fallback{Value: kernel.Text(f.Base), Origin: kernel.OriginDefault}, len(req.Branches) > 0
		},
	}
}

type store struct{ entries []string }

func (s *store) add(name string) func(context.Context) error {
	return func(context.Context) error { s.entries = append(s.entries, name); return nil }
}

func (s *store) remove(name string) func(context.Context) error {
	return func(context.Context) error {
		s.entries = slices.DeleteFunc(s.entries, func(entry string) bool { return entry == name })
		return nil
	}
}

func (s *store) snapshot() string { return strings.Join(s.entries, ",") }
