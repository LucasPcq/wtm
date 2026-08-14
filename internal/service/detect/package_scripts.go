package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// PackageJSONScripts returns all package.json scripts under projectDir,
// including workspace packages when a workspace is declared at the root
// (pnpm-workspace.yaml or a package.json "workspaces" field).
// Results are ordered: root scripts first (alphabetically), then workspace
// packages (alphabetically by dir, then by script name within each package).
// Returns nil if no package.json is found at the project root.
func PackageJSONScripts(projectDir string) []domain.PackageScript {
	root := readPackageScripts(projectDir, "")
	if root == nil {
		return nil
	}

	scripts := append([]domain.PackageScript{}, root...)

	for _, wsDir := range WorkspacePackages(projectDir) {
		ws := readPackageScripts(filepath.Join(projectDir, wsDir), wsDir)
		scripts = append(scripts, ws...)
	}

	return scripts
}

type packageJSONFile struct {
	Name    string            `json:"name"`
	Scripts map[string]string `json:"scripts"`
}

func readPackageScripts(dir, workspace string) []domain.PackageScript {
	data, err := os.ReadFile(filepath.Join(dir, domain.PackageJSONFile))
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
