package worktree

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

// writeMetadata lays a meta.json down as a fixture, bypassing updateMetadata.
func writeMetadata(metaDir string, metadata domain.WorktreeMetadata) error {
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(metaDir, domain.MetaFileName), data, 0o644)
}
