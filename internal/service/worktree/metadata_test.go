package worktree

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestMetadataAtANewerSchemaIsReadButNeverRewritten(t *testing.T) {
	stateDir := t.TempDir()
	metaDir := rules.WorktreeMetaDir(stateDir, "feat/a")
	if err := writeMetadata(metaDir, domain.WorktreeMetadata{Schema: 99, SourceBranch: "main"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(metaDir, domain.MetaFileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	meta, err := loadMetadata(stateDir, "feat/a")
	if err != nil || meta.SourceBranch != "main" {
		t.Fatalf("loadMetadata = %+v, %v", meta, err)
	}

	_, err = setSourceBranch(t.Context(), setSourceBranchParams{StateDir: stateDir, Branch: "feat/a", NewParent: "dev"})
	if !errors.Is(err, domainstatefile.ErrSchemaNewer) {
		t.Fatalf("error = %v, want %v", err, domainstatefile.ErrSchemaNewer)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("meta.json rewritten:\n%s", after)
	}
}

func TestRecordNamespacesLosesNoConcurrentRecord(t *testing.T) {
	stateDir := t.TempDir()
	if err := writeMetadata(rules.WorktreeMetaDir(stateDir, "feat/a"), domain.WorktreeMetadata{SourceBranch: "main"}); err != nil {
		t.Fatal(err)
	}

	const jobs = 16
	var wg sync.WaitGroup
	for i := range jobs {
		wg.Go(func() {
			if err := RecordNamespaces(t.Context(), RecordNamespacesParams{StateDir: stateDir, Branch: "feat/a", Jobs: []string{"db" + strconv.Itoa(i)}}); err != nil {
				t.Errorf("RecordNamespaces: %v", err)
			}
		})
	}
	wg.Wait()

	meta, err := loadMetadata(stateDir, "feat/a")
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Namespaces) != jobs {
		t.Errorf("recorded %d namespaces, want %d: %v", len(meta.Namespaces), jobs, meta.Namespaces)
	}
	if meta.SourceBranch != "main" || !slices.Contains(meta.Namespaces, "db0") {
		t.Errorf("record = %+v", meta)
	}
}
