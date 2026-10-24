// Package configtest is a stateful fake of config.Project, the project's config.toml and run.toml, in memory.
package configtest

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

const configSchema = 1

// A Fake stands in for config.Project, in memory: config.toml as Load returns
// it, with no global config, and run.toml. It keeps config.Project's contract (no
// config.toml is project.ErrConfigNotFound, one that does not parse a
// *project.InvalidConfigError, a newer schema is never rewritten, a Remember
// that changes nothing reports no change) and records every call. A test seeds
// it with [NewFake], makes a method fail through FakeParams.Fail, and reads back
// what was called with Calls. It is safe for concurrent use.
type Fake struct {
	mu         sync.Mutex
	config     *project.Config
	run        project.Run
	schema     int
	unreadable bool
	fail       map[string]error
	calls      []string
}

// FakeParams is what a [Fake] starts with.
type FakeParams struct {
	Config     *project.Config  // config.toml as Load returns it, its defaults applied; nil when there is none
	Run        project.Run      // run.toml as LoadRun returns it, Declared when there is one
	Schema     int              // the schema config.toml carries, to simulate a newer wtm
	Unreadable bool             // config.toml exists but does not parse
	Fail       map[string]error // the error a method returns, by method name
}

// NewFake returns a Fake holding copies of what params seeds.
func NewFake(params FakeParams) *Fake {
	fake := &Fake{run: params.Run, schema: params.Schema, unreadable: params.Unreadable, fail: params.Fail}
	if params.Config != nil {
		config := *params.Config
		config.Remembered = maps.Clone(config.Remembered)
		fake.config = &config
	}
	return fake
}

// Calls returns the name of every method called so far, in order.
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// Remove takes config.toml away, as a user deleting it would.
func (f *Fake) Remove() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.config = nil
	f.unreadable = false
}

// Load returns config.toml and an empty global config as config.Project.Load does:
// project.ErrConfigNotFound, wrapped, without a config.toml, a
// *project.InvalidConfigError when it does not parse.
func (f *Fake) Load() (project.Config, project.Global, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Load"); err != nil {
		return project.Config{}, project.Global{}, err
	}

	if f.config == nil {
		return project.Config{}, project.Global{}, fmt.Errorf("configtest: %w", project.ErrConfigNotFound)
	}
	if f.unreadable {
		return project.Config{}, project.Global{}, &project.InvalidConfigError{Reason: errors.New("configtest: config.toml does not parse")}
	}
	return *f.config, project.Global{}, nil
}

// LoadRun returns run.toml as config.Project.LoadRun does.
func (f *Fake) LoadRun() (project.Run, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("LoadRun"); err != nil {
		return project.Run{}, err
	}
	return f.run, nil
}

// Remember merges answers over config.toml's remembered answers as
// config.Project.Remember does: no answers write nothing,
// project.ErrConfigNotFound without a config.toml, an error when it does not
// parse or carries a newer schema, no change when every answer is already
// remembered.
func (f *Fake) Remember(_ context.Context, answers map[string]string) (domain.Written, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Remember"); err != nil {
		return domain.Written{}, err
	}

	if len(answers) == 0 {
		return domain.Written{}, nil
	}
	if f.config == nil {
		return domain.Written{}, project.ErrConfigNotFound
	}
	// A file that does not parse reads no schema: config.Project reports the parse error.
	if f.unreadable {
		return domain.Written{}, errors.New("configtest: config.toml does not parse")
	}
	if err := statefile.Refusal(statefile.Schema{Path: "config.toml", Found: f.schema, Supported: configSchema}); err != nil {
		return domain.Written{}, err
	}

	remembered := map[string]string{}
	maps.Copy(remembered, f.config.Remembered)
	maps.Copy(remembered, answers)
	if maps.Equal(remembered, f.config.Remembered) {
		return domain.Written{}, nil
	}

	f.config.Remembered = remembered
	return domain.Written{Changed: true}, nil
}

// enter records the call of method and returns the error FakeParams.Fail sets
// for it, if any.
func (f *Fake) enter(method string) error {
	f.calls = append(f.calls, method)
	return f.fail[method]
}
