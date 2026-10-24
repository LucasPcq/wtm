// Package storetest is a stateful fake of the store, in memory.
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
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const metaSchema = 1

// A Fake stands in for store.Meta and store.EnvFiles, in memory: the meta.json
// records by branch and the files by path. It keeps their contracts (a write
// that changes nothing reports no change, a newer schema is refused, a record
// that does not read is rewritten) and records every call. A test seeds it with
// [NewFake], makes a method fail through FakeParams.Fail, and reads back what
// was written with Meta, File and Calls. It is safe for concurrent use.
type Fake struct {
	mu         sync.Mutex
	meta       map[string]worktree.Meta
	unreadable map[string]bool
	files      map[string]string
	fail       map[string]error
	calls      []string
}

// FakeParams is what a [Fake] starts with.
type FakeParams struct {
	Meta       map[string]worktree.Meta // the records by branch
	Unreadable []string                 // branches whose record exists but does not decode
	Files      map[string]string        // the content of the files by path
	Fail       map[string]error         // the error a method returns, by method name
}

// NewFake returns a Fake holding copies of what params seeds.
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
	return fake
}

// Calls returns every call made so far, in order, each as its method name and
// its branch or path ("Load feat/a", "Edit /w/.env").
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// File returns the content of the file at path and reports whether it exists.
func (f *Fake) File(path string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	content, ok := f.files[path]
	return content, ok
}

// Meta returns the record of branch and reports whether there is one that reads.
func (f *Fake) Meta(branch string) (worktree.Meta, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	meta, ok := f.meta[branch]
	return meta, ok && !f.unreadable[branch]
}

// Load returns the record of branch as store.Meta.Load does: an error for a
// branch that maps onto no directory, a record that does not read, or one that
// is missing (wrapping os.ErrNotExist).
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

// List returns the branches that hold a record, readable or not, sorted, as
// store.Meta.List does.
func (f *Fake) List() ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("List", ""); err != nil {
		return nil, err
	}

	branches := slices.Collect(maps.Keys(f.meta))
	for branch := range f.unreadable {
		if _, ok := f.meta[branch]; !ok {
			branches = append(branches, branch)
		}
	}
	slices.Sort(branches)
	return branches, nil
}

// Update applies update.Change to the record of update.Branch as
// store.Meta.Update does: from an empty record when there is none or it does
// not read, refusing a newer schema, and reporting no change when the record
// would be written byte for byte as it is.
func (f *Fake) Update(_ context.Context, update worktree.MetaUpdate) (domain.Written, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Update", update.Branch); err != nil {
		return domain.Written{}, err
	}

	if err := segmentOf(update.Branch); err != nil {
		return domain.Written{}, err
	}

	stored, err := f.stored(update.Branch)
	if err != nil {
		return domain.Written{}, err
	}

	next := stored
	if err := update.Change(&next); err != nil {
		return domain.Written{}, err
	}

	next.Schema = 0
	written, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return domain.Written{}, err
	}

	previous, held := f.meta[update.Branch]
	before, _ := json.MarshalIndent(previous, "", "  ")
	if held && !f.unreadable[update.Branch] && bytes.Equal(before, written) {
		return domain.Written{}, nil
	}

	// Kept as JSON reads it back, so a test sees what a real reload would.
	var reread worktree.Meta
	if err := json.Unmarshal(written, &reread); err != nil {
		return domain.Written{}, err
	}

	f.meta[update.Branch] = reread
	delete(f.unreadable, update.Branch)
	return domain.Written{Changed: true}, nil
}

// segmentOf returns an error for a branch name that store.Meta maps onto no
// directory.
func segmentOf(branch string) error {
	if _, ok := worktree.MetaDir(worktree.MetaDirParams{StateDir: "/", Branch: branch}); !ok {
		return fmt.Errorf("storetest: no meta.json path for branch %q", branch)
	}
	return nil
}

// stored returns the record an update starts from, as store's storedMeta does.
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

// LockOrdinals runs do, reporting the lock as held; it does not exclude
// anything, since do calls back into the fake.
func (f *Fake) LockOrdinals(_ context.Context, do func() error) (bool, error) {
	f.mu.Lock()
	err := f.enter("LockOrdinals", "")
	f.mu.Unlock()
	if err != nil {
		return false, err
	}

	return true, do()
}

// Read returns the lines of the file at path, none when it does not exist.
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

// Copy copies the files request names between the fake's files as
// store.EnvFiles.Copy does, listing only the files whose content changed.
func (f *Fake) Copy(_ context.Context, request env.CopyRequest) (env.Copied, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Copy", request.TargetDir); err != nil {
		return env.Copied{}, err
	}

	sources, warnings, err := env.Sources(request, f.exists)
	if err != nil {
		return env.Copied{}, err
	}

	copied := env.Copied{Warnings: warnings}
	for _, source := range sources {
		if current, held := f.files[source.To]; held && current == f.files[source.From] {
			continue
		}
		f.files[source.To] = f.files[source.From]
		copied.Files = append(copied.Files, source.To)
	}
	return copied, nil
}

// Edit applies edit to the file at edit.Path as store.EnvFiles.Edit does.
func (f *Fake) Edit(_ context.Context, edit dotenv.Edit) (domain.Written, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Edit", edit.Path); err != nil {
		return domain.Written{}, err
	}

	var current []byte
	if content, ok := f.files[edit.Path]; ok {
		current = []byte(content)
	}

	next, write := edit.Over(current)
	if !write {
		return domain.Written{}, nil
	}

	f.files[edit.Path] = string(next)
	return domain.Written{Changed: true}, nil
}

func (f *Fake) exists(path string) (bool, error) {
	_, ok := f.files[path]
	return ok, nil
}

// enter records the call of method on arg and returns the error FakeParams.Fail
// sets for method, if any.
func (f *Fake) enter(method, arg string) error {
	f.calls = append(f.calls, method+" "+arg)
	return f.fail[method]
}

func unreadable(branch string) error {
	return fmt.Errorf("storetest: the record of %s does not read", branch)
}
