package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// DockerFilesConfigured returns the set of detected docker-compose files that
// already back a job in run, matched on the "-f <file> " fragment that
// BuildDockerJobs emits.
func DockerFilesConfigured(run domain.RunConfig, files []string) map[string]bool {
	configured := map[string]bool{}
	for _, f := range files {
		needle := DockerComposeFileFlag(f)
		for _, job := range run.Jobs {
			if strings.Contains(job.Cmd, needle) {
				configured[f] = true
				break
			}
		}
	}
	return configured
}

// ScriptsConfigured returns the set of detected script indices that already back
// a job in run, matched on the "<pm> run <name>" command and cwd that
// BuildScriptJobs emits.
func ScriptsConfigured(run domain.RunConfig, scripts []domain.PackageScript, pm domain.PackageManager) map[int]bool {
	configured := map[int]bool{}
	for i, s := range scripts {
		cmd := ScriptJobCmd(pm, s.Name)
		cwd := ScriptJobCwd(s.Workspace)
		for _, job := range run.Jobs {
			if job.Cmd == cmd && job.Cwd == cwd {
				configured[i] = true
				break
			}
		}
	}
	return configured
}
