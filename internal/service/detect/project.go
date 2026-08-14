package detect

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// ProjectEnvironment runs all detection probes for a project directory and
// returns a fully-populated InitDetectionResult. Package script kinds are
// pre-classified so the wizard can drive selection without further logic.
func ProjectEnvironment(dir string) domain.InitDetectionResult {
	pm := PackageManager(dir)
	scripts := PackageJSONScripts(dir)
	for i := range scripts {
		scripts[i].Kind = rules.ClassifyScriptKind(scripts[i].Name)
	}
	return domain.InitDetectionResult{
		BaseBranch:         BaseBranch(dir),
		Branches:           Branches(dir),
		EnvFiles:           EnvFiles(dir),
		PackageManager:     pm,
		InstallCommand:     rules.InstallCommand(pm),
		DockerComposeFiles: DockerComposeFiles(dir),
		DockerComposeCmd:   DockerComposeCommand(),
		Monorepo:           Monorepo(dir),
		PackageScripts:     scripts,
	}
}
