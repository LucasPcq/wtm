// Package storetest is a stateful fake of the store and of the answers config.toml remembers, in memory.
package storetest

import (
	"context"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/envcopy"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

const metaSchema = 1

type Fake struct {
	mu         sync.Mutex
	meta       map[string]worktree.Meta
	unreadable map[string]bool
	config     *project.Config
	files      map[string]string
	fail       map[string]error
	calls      []string
}

type FakeParams struct {
	Meta       map[string]worktree.Meta
	Unreadable []string
	Config     *project.Config
	Files      map[string]string
	Fail       map[string]error
}

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
		fake.meta = map[string]worktree.Meta{}
	}
	if fake.files == nil {
		fake.files = map[string]string{}
	}
	if params.Config != nil {
		config := *params.Config
		config.Remembered = maps.Clone(config.Remembered)
		fake.config = &config
	}
	return fake
}

func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func (f *Fake) Config() (project.Config, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.config == nil {
		return project.Config{}, false
	}
	return *f.config, true
}

func (f *Fake) File(path string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	content, ok := f.files[path]
	return content, ok
}

func (f *Fake) Meta(branch string) (worktree.Meta, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	meta, ok := f.meta[branch]
	return meta, ok && !f.unreadable[branch]
}

func (f *Fake) Load(branch string) (worktree.Meta, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Load", branch); err != nil {
		return worktree.Meta{}, err
	}
	if f.unreadable[branch] {
		return worktree.Meta{}, unreadable(branch)
	}
	meta, ok := f.meta[branch]
	if !ok {
		return worktree.Meta{}, fmt.Errorf("storetest: no record of %s: %w", branch, os.ErrNotExist)
	}
	return meta, nil
}

func (f *Fake) Records() ([]worktree.MetaRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Records", ""); err != nil {
		return nil, err
	}
	var records []worktree.MetaRecord
	for _, branch := range slices.Sorted(maps.Keys(f.meta)) {
		if !f.unreadable[branch] {
			records = append(records, worktree.MetaRecord{Branch: branch, Meta: f.meta[branch]})
		}
	}
	return records, nil
}

func (f *Fake) Update(_ context.Context, update worktree.MetaUpdate) (wtm.Written, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Update", update.Branch); err != nil {
		return wtm.Written{}, err
	}
	stored, err := f.stored(update)
	if err != nil || stored == nil {
		return wtm.Written{}, err
	}
	next := *stored
	if err := update.Change(&next); err != nil {
		return wtm.Written{}, err
	}
	next.Schema = statefile.Stamp(metaSchema)
	previous, held := f.meta[update.Branch]
	if held && !f.unreadable[update.Branch] && reflect.DeepEqual(previous, next) {
		return wtm.Written{}, nil
	}
	f.meta[update.Branch] = next
	delete(f.unreadable, update.Branch)
	return wtm.Written{Changed: true}, nil
}

func (f *Fake) stored(update worktree.MetaUpdate) (*worktree.Meta, error) {
	if f.unreadable[update.Branch] && update.Missing == worktree.MissingCreate {
		return &worktree.Meta{}, nil
	}
	if f.unreadable[update.Branch] {
		return nil, unreadable(update.Branch)
	}
	stored, ok := f.meta[update.Branch]
	switch {
	case !ok && update.Missing == worktree.MissingSkip:
		return nil, nil
	case !ok && update.Missing == worktree.MissingRefuse:
		return nil, fmt.Errorf("storetest: no record of %s: %w", update.Branch, os.ErrNotExist)
	}
	if err := statefile.Refusal(statefile.Schema{Path: update.Branch, Found: stored.Schema, Supported: metaSchema}); err != nil {
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

func (f *Fake) Remember(_ context.Context, answers map[string]string) (wtm.Written, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Remember", ""); err != nil {
		return wtm.Written{}, err
	}
	if len(answers) == 0 {
		return wtm.Written{}, nil
	}
	if f.config == nil {
		return wtm.Written{}, project.ErrConfigNotFound
	}
	remembered := maps.Clone(f.config.Remembered)
	if remembered == nil {
		remembered = map[string]string{}
	}
	maps.Copy(remembered, answers)
	if maps.Equal(remembered, f.config.Remembered) {
		return wtm.Written{}, nil
	}
	f.config.Remembered = remembered
	return wtm.Written{Changed: true}, nil
}

func (f *Fake) Read(path string) ([]dotenv.Line, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Read", path); err != nil {
		return nil, err
	}
	content, ok := f.files[path]
	if !ok {
		return nil, nil
	}
	return dotenv.Parse([]byte(content)), nil
}

func (f *Fake) Copy(_ context.Context, request envcopy.Request) (envcopy.Copied, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Copy", request.TargetDir); err != nil {
		return envcopy.Copied{}, err
	}
	candidates, err := envcopy.Candidates(request)
	if err != nil {
		return envcopy.Copied{}, err
	}
	sources, warnings, err := envcopy.Sources(envcopy.SourcesParams{Request: request, Present: f.present(candidates)})
	if err != nil {
		return envcopy.Copied{}, err
	}
	copied := envcopy.Copied{Warnings: warnings}
	for _, source := range sources {
		if current, held := f.files[source.To]; held && current == f.files[source.From] {
			continue
		}
		f.files[source.To] = f.files[source.From]
		copied.Files = append(copied.Files, source.To)
	}
	return copied, nil
}

func (f *Fake) Edit(_ context.Context, edit dotenv.Edit) (wtm.Written, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Edit", edit.Path); err != nil {
		return wtm.Written{}, err
	}
	var current []byte
	if content, ok := f.files[edit.Path]; ok {
		current = []byte(content)
	}
	next, write := edit.Over(current)
	if !write {
		return wtm.Written{}, nil
	}
	f.files[edit.Path] = string(next)
	return wtm.Written{Changed: true}, nil
}

func (f *Fake) present(paths []string) map[string]bool {
	present := map[string]bool{}
	for _, path := range paths {
		_, present[path] = f.files[path]
	}
	return present
}

func (f *Fake) enter(method, arg string) error {
	f.calls = append(f.calls, method+" "+arg)
	return f.fail[method]
}

func unreadable(branch string) error {
	return fmt.Errorf("storetest: the record of %s does not read", branch)
}
