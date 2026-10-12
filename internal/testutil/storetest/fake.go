// Package storetest doubles the ports of store: the state files in memory.
package storetest

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
	"github.com/LucasPcq/wtm/internal/store"
)

// Fake is the state files in memory: what its writes change, its reads give
// back, and it refuses what the disk refuses.
type Fake struct {
	mu     sync.Mutex
	meta   map[string]domain.WorktreeMetadata
	config *domain.ProjectConfig
	files  map[string]string
	fail   map[string]error
	calls  []string
}

// FakeParams starts the fake: the records by branch, config.toml (nil when
// there is none), every file by path — the .env files and the sources a copy
// reads — and the error a method returns instead of running, keyed by its name.
type FakeParams struct {
	Meta   map[string]domain.WorktreeMetadata
	Config *domain.ProjectConfig
	Files  map[string]string
	Fail   map[string]error
}

var (
	_ store.Meta          = (*Fake)(nil)
	_ store.ProjectConfig = (*Fake)(nil)
	_ store.EnvFiles      = (*Fake)(nil)
)

func NewFake(params FakeParams) *Fake {
	fake := &Fake{
		meta:  maps.Clone(params.Meta),
		files: maps.Clone(params.Files),
		fail:  params.Fail,
	}
	if fake.meta == nil {
		fake.meta = map[string]domain.WorktreeMetadata{}
	}
	if fake.files == nil {
		fake.files = map[string]string{}
	}
	if params.Config != nil {
		config := *params.Config
		fake.config = &config
	}
	return fake
}

// Calls are the methods run, writes and reads, each with its main argument.
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// Config is config.toml as the fake holds it, false when there is none.
func (f *Fake) Config() (domain.ProjectConfig, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.config == nil {
		return domain.ProjectConfig{}, false
	}
	return *f.config, true
}

// File is the content of a file the fake holds.
func (f *Fake) File(path string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	content, ok := f.files[path]
	return content, ok
}

func (f *Fake) Load(branch string) (domain.WorktreeMetadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Load", branch); err != nil {
		return domain.WorktreeMetadata{}, err
	}
	meta, ok := f.meta[branch]
	if !ok {
		return domain.WorktreeMetadata{}, fmt.Errorf("storetest: no record of %s: %w", branch, os.ErrNotExist)
	}
	return meta, nil
}

func (f *Fake) Records() ([]store.MetaRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Records", ""); err != nil {
		return nil, err
	}
	var records []store.MetaRecord
	for _, branch := range slices.Sorted(maps.Keys(f.meta)) {
		records = append(records, store.MetaRecord{Branch: branch, Meta: f.meta[branch]})
	}
	return records, nil
}

func (f *Fake) Update(_ context.Context, update store.MetaUpdate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Update", update.Branch); err != nil {
		return err
	}
	stored, ok := f.meta[update.Branch]
	switch {
	case !ok && update.Missing == store.MissingSkip:
		return nil
	case !ok && update.Missing == store.MissingRefuse:
		return fmt.Errorf("storetest: no record of %s: %w", update.Branch, os.ErrNotExist)
	}
	if err := statefile.Refusal(statefile.Schema{Path: update.Branch, Found: stored.Schema, Supported: domainstatefile.MetaSchema}); err != nil {
		return err
	}
	if err := update.Change(&stored); err != nil {
		return err
	}
	stored.Schema = statefile.Written(domainstatefile.MetaSchema)
	f.meta[update.Branch] = stored
	return nil
}

func (f *Fake) Purge(branch string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Purge", branch); err != nil {
		return err
	}
	delete(f.meta, branch)
	return nil
}

func (f *Fake) Remember(_ context.Context, change rules.RememberedChange) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Remember", ""); err != nil {
		return err
	}
	if len(change.Remember) == 0 && len(change.Forget) == 0 {
		return nil
	}
	if f.config == nil {
		return domain.ErrConfigNotFound
	}
	if err := statefile.Refusal(statefile.Schema{Path: domain.ConfigFileName, Found: f.config.Schema, Supported: domainstatefile.ProjectConfigSchema}); err != nil {
		return err
	}
	f.config.Wizard.Remembered = rules.ApplyRemembered(f.config.Wizard.Remembered, change)
	return nil
}

func (f *Fake) Copy(_ context.Context, request store.EnvCopy) ([]store.EnvWarning, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Copy", request.TargetDir); err != nil {
		return nil, err
	}
	sources, warnings, err := store.CopySources(store.CopySourcesParams{Request: request, Exists: f.exists})
	if err != nil {
		return nil, err
	}
	for _, source := range sources {
		f.files[source.Dst] = f.files[source.Src]
	}
	return warnings, nil
}

func (f *Fake) Edit(_ context.Context, edit store.EnvEdit) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Edit", edit.Path); err != nil {
		return false, err
	}
	var current []byte
	if content, ok := f.files[edit.Path]; ok {
		current = []byte(content)
	}
	next, write := store.EditEnv(store.EditEnvParams{Edit: edit, Current: current})
	if !write {
		return false, nil
	}
	f.files[edit.Path] = string(next)
	return true, nil
}

func (f *Fake) exists(path string) bool {
	_, ok := f.files[path]
	return ok
}

func (f *Fake) enter(method, arg string) error {
	f.calls = append(f.calls, method+" "+arg)
	return f.fail[method]
}
