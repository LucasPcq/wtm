package agents

import (
	"embed"
	"io/fs"
	"strings"
)

const (
	skillEntryFile     = "SKILL.md"
	skillReferencesDir = "references"
	skillAssetRoot     = "assets/using-wtm"
)

// skillFS is the using-wtm skill shipped with wtm: SKILL.md, loaded whenever
// the skill triggers, and the reference files it points at, read on demand.
//
//go:embed assets/using-wtm
var skillFS embed.FS

// skillFiles maps each file of the skill, by its path inside the skill
// directory (slash-separated), to its content.
func skillFiles() map[string]string {
	files := map[string]string{}
	_ = fs.WalkDir(skillFS, skillAssetRoot, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, readErr := skillFS.ReadFile(name)
		if readErr != nil {
			return readErr
		}
		files[strings.TrimPrefix(name, skillAssetRoot+"/")] = string(content)
		return nil
	})
	return files
}
