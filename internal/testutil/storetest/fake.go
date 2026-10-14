// Package storetest doubles the ports of store: the state files in memory.
package storetest

import (
	"context"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/rules/envcopy"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
	"github.com/LucasPcq/wtm/internal/store"
)

// Fake is the state files in memory: what its writes change, its reads give
// back, and it refuses what the disk refuses.
type Fake struct {
	mu         sync.Mutex
	meta       map[string]domain.WorktreeMetadata
	unreadable map[string]bool
	config     *domain.ProjectConfig
	files      map[string]string
	fail       map[string]error
	calls      []string
}

// FakeParams starts the fake: the records by branch, the branches whose
// meta.json does not read, config.toml (nil when there is none), every file by
// path — the .env files and the sources a copy reads — and the error a method
// returns instead of running, keyed by its name.
type FakeParams struct {
	Meta       map[string]domain.WorktreeMetadata
	Unreadable []string
	Config     *domain.ProjectConfig
	Files      map[string]string
	Fail       map[string]error
}

var (
	_ store.Meta          = (*Fake)(nil)
	_ store.ProjectConfig = (*Fake)(nil)
	_ store.EnvFiles      = (*Fake)(nil)
)

func NewFake(params FakeParams) *Fake {
	fake := &Fake{
		meta:       maps.Clone(params.Meta),
		unreadable: map[string]bool{},
		files:      maps.Clone(params.Files),
		fail:       params.Fail,
	}
	for _, branch := range params.Unreadable {
		fake.unreadable[branch] = true
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
	if f.unreadable[branch] {
		return domain.WorktreeMetadata{}, unreadable(branch)
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
		if f.unreadable[branch] {
			continue
		}
		records = append(records, store.MetaRecord{Branch: branch, Meta: f.meta[branch]})
	}
	return records, nil
}

func (f *Fake) Update(_ context.Context, update store.MetaUpdate) (store.Written, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Update", update.Branch); err != nil {
		return store.Written{}, err
	}
	stored, err := f.stored(update)
	if err != nil || stored == nil {
		return store.Written{}, err
	}
	next := *stored
	if err := update.Change(&next); err != nil {
		return store.Written{}, err
	}
	next.Schema = statefile.Written(domainstatefile.MetaSchema)
	previous, held := f.meta[update.Branch]
	if held && !f.unreadable[update.Branch] && reflect.DeepEqual(previous, next) {
		return store.Written{}, nil
	}
	f.meta[update.Branch] = next
	delete(f.unreadable, update.Branch)
	return store.Written{Changed: true}, nil
}

// stored is the record Update starts from, nil when it writes nothing.
func (f *Fake) stored(update store.MetaUpdate) (*domain.WorktreeMetadata, error) {
	if f.unreadable[update.Branch] {
		if update.Missing == store.MissingCreate {
			return &domain.WorktreeMetadata{}, nil
		}
		return nil, unreadable(update.Branch)
	}
	stored, ok := f.meta[update.Branch]
	switch {
	case !ok && update.Missing == store.MissingSkip:
		return nil, nil
	case !ok && update.Missing == store.MissingRefuse:
		return nil, fmt.Errorf("storetest: no record of %s: %w", update.Branch, os.ErrNotExist)
	}
	if err := statefile.Refusal(statefile.Schema{Path: update.Branch, Found: stored.Schema, Supported: domainstatefile.MetaSchema}); err != nil {
		return nil, err
	}
	return &stored, nil
}

func (f *Fake) Purge(branch string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Purge", branch); err != nil {
		return err
	}
	delete(f.meta, branch)
	delete(f.unreadable, branch)
	return nil
}

func (f *Fake) Remember(_ context.Context, change rules.RememberedChange) (store.Written, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Remember", ""); err != nil {
		return store.Written{}, err
	}
	if len(change.Remember) == 0 && len(change.Forget) == 0 {
		return store.Written{}, nil
	}
	if f.config == nil {
		return store.Written{}, domain.ErrConfigNotFound
	}
	if err := statefile.Refusal(statefile.Schema{Path: domain.ConfigFileName, Found: f.config.Schema, Supported: domainstatefile.ProjectConfigSchema}); err != nil {
		return store.Written{}, err
	}
	remembered := rules.ApplyRemembered(f.config.Wizard.Remembered, change)
	if reflect.DeepEqual(remembered, f.config.Wizard.Remembered) {
		return store.Written{}, nil
	}
	f.config.Wizard.Remembered = remembered
	return store.Written{Changed: true}, nil
}

func (f *Fake) Read(path string) ([]domain.EnvLine, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Read", path); err != nil {
		return nil, err
	}
	content, ok := f.files[path]
	if !ok {
		return nil, nil
	}
	return rules.ParseEnv(content), nil
}

func (f *Fake) Template(dir string, file domain.EnvFile) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Template", dir); err != nil {
		return "", err
	}
	candidates := envcopy.TemplateCandidates(dir, file)
	return envcopy.Template(candidates, f.present(candidates)), nil
}

func (f *Fake) Copy(_ context.Context, request envcopy.Request) (store.EnvCopied, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Copy", request.TargetDir); err != nil {
		return store.EnvCopied{}, err
	}
	candidates, err := envcopy.Candidates(request)
	if err != nil {
		return store.EnvCopied{}, err
	}
	sources, warnings, err := envcopy.Sources(envcopy.SourcesParams{Request: request, Present: f.present(candidates)})
	if err != nil {
		return store.EnvCopied{}, err
	}
	copied := store.EnvCopied{Warnings: warnings}
	for _, source := range sources {
		current, held := f.files[source.Dst]
		if held && current == f.files[source.Src] {
			continue
		}
		f.files[source.Dst] = f.files[source.Src]
		copied.Files = append(copied.Files, source.Dst)
	}
	return copied, nil
}

func (f *Fake) Edit(_ context.Context, edit store.EnvEdit) (store.Written, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Edit", edit.Path); err != nil {
		return store.Written{}, err
	}
	var current []byte
	if content, ok := f.files[edit.Path]; ok {
		current = []byte(content)
	}
	next, write := store.EditEnv(store.EditEnvParams{Edit: edit, Current: current})
	if !write {
		return store.Written{}, nil
	}
	f.files[edit.Path] = string(next)
	return store.Written{Changed: true}, nil
}

func (f *Fake) present(paths []string) map[string]bool {
	present := map[string]bool{}
	for _, path := range paths {
		_, present[path] = f.files[path]
	}
	return present
}

func unreadable(branch string) error {
	return fmt.Errorf("storetest: the record of %s does not read", branch)
}

func (f *Fake) enter(method, arg string) error {
	f.calls = append(f.calls, method+" "+arg)
	return f.fail[method]
}
