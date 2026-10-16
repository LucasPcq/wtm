// Package storetest is a stateful fake of the store and of the answers config.toml remembers, in memory.
package storetest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
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
	mu               sync.Mutex
	meta             map[string]worktree.Meta
	unreadable       map[string]bool
	config           *project.Config
	configSchema     int
	configUnreadable bool
	files            map[string]string
	fail             map[string]error
	calls            []string
}

type FakeParams struct {
	Meta             map[string]worktree.Meta
	Unreadable       []string
	Config           *project.Config
	ConfigSchema     int
	ConfigUnreadable bool
	Files            map[string]string
	Fail             map[string]error
}

func NewFake(params FakeParams) *Fake {
	fake := &Fake{
		meta:             maps.Clone(params.Meta),
		unreadable:       map[string]bool{},
		files:            maps.Clone(params.Files),
		fail:             params.Fail,
		configSchema:     params.ConfigSchema,
		configUnreadable: params.ConfigUnreadable,
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
	if err := segmentOf(branch); err != nil {
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

func (f *Fake) Update(_ context.Context, update worktree.MetaUpdate) (wtm.Written, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Update", update.Branch); err != nil {
		return wtm.Written{}, err
	}
	if err := segmentOf(update.Branch); err != nil {
		return wtm.Written{}, err
	}
	stored, err := f.stored(update.Branch)
	if err != nil {
		return wtm.Written{}, err
	}
	next := stored
	if err := update.Change(&next); err != nil {
		return wtm.Written{}, err
	}
	next.Schema = 0
	written, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return wtm.Written{}, err
	}
	previous, held := f.meta[update.Branch]
	before, _ := json.MarshalIndent(previous, "", "  ")
	if held && !f.unreadable[update.Branch] && bytes.Equal(before, written) {
		return wtm.Written{}, nil
	}
	var reread worktree.Meta
	if err := json.Unmarshal(written, &reread); err != nil {
		return wtm.Written{}, err
	}
	f.meta[update.Branch] = reread
	delete(f.unreadable, update.Branch)
	return wtm.Written{Changed: true}, nil
}

func segmentOf(branch string) error {
	if _, ok := worktree.MetaDir(worktree.MetaDirParams{StateDir: "/", Branch: branch}); !ok {
		return fmt.Errorf("storetest: no meta.json path for branch %q", branch)
	}
	return nil
}

func (f *Fake) stored(branch string) (worktree.Meta, error) {
	stored, ok := f.meta[branch]
	if !ok || f.unreadable[branch] {
		return worktree.Meta{}, nil
	}
	if err := statefile.Refusal(statefile.Schema{Path: branch, Found: stored.Schema, Supported: metaSchema}); err != nil {
		return worktree.Meta{}, err
	}
	return stored, nil
}

func (f *Fake) LockOrdinals(_ context.Context, do func() error) (bool, error) {
	f.mu.Lock()
	err := f.enter("LockOrdinals", "")
	f.mu.Unlock()
	if err != nil {
		return false, err
	}
	return true, do()
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
	if f.configUnreadable {
		return wtm.Written{}, fmt.Errorf("storetest: config.toml does not parse")
	}
	if err := statefile.Refusal(statefile.Schema{Path: "config.toml", Found: f.configSchema, Supported: 1}); err != nil {
		return wtm.Written{}, err
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
	sources, warnings, err := envcopy.Sources(envcopy.SourcesParams{Request: request, Exists: f.exists})
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

func (f *Fake) exists(path string) (bool, error) {
	_, ok := f.files[path]
	return ok, nil
}

func (f *Fake) enter(method, arg string) error {
	f.calls = append(f.calls, method+" "+arg)
	return f.fail[method]
}

func unreadable(branch string) error {
	return fmt.Errorf("storetest: the record of %s does not read", branch)
}
