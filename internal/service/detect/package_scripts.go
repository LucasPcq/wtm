package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// PackageJSONScripts returns all package.json scripts under projectDir: the
// root manifest's, then each workspace package's, ordered by directory and by
// script name within a package.
//
// A missing root manifest is not an empty answer. A repo whose packages all sit
// in subdirectories still declares its workspace, and bailing on the root left
// it with nothing detected at all (LUC-208).
func PackageJSONScripts(projectDir string) []domain.PackageScript {
	scripts := append([]domain.PackageScript{}, readPackageScripts(projectDir, "")...)

	for _, wsDir := range WorkspacePackages(projectDir) {
		ws := readPackageScripts(filepath.Join(projectDir, wsDir), wsDir)
		scripts = append(scripts, ws...)
	}

	if len(scripts) == 0 {
		return nil
	}
	return scripts
}

type packageJSONFile struct {
	Name    string            `json:"name"`
	Scripts map[string]string `json:"scripts"`
}

func readPackageScripts(dir, workspace string) []domain.PackageScript {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil
	}

	var pkg packageJSONFile
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	pkgName := rules.StripScope(pkg.Name)
	if pkgName == "" {
		pkgName = filepath.Base(dir)
	}

	names := make([]string, 0, len(pkg.Scripts))
	for name := range pkg.Scripts {
		names = append(names, name)
	}
	sort.Strings(names)

	scripts := make([]domain.PackageScript, 0, len(names))
	for _, name := range names {
		scripts = append(scripts, domain.PackageScript{
			Name:      name,
			Cmd:       pkg.Scripts[name],
			Workspace: workspace,
			PkgName:   pkgName,
		})
	}

	return scripts
}
