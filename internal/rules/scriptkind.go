package rules

import "github.com/LucasPcq/wtm/internal/domain"

type ProposedScriptKindParams struct {
	Script         domain.PackageScript
	Config         domain.RunConfig
	PackageManager domain.PackageManager
}

// ProposedScriptKind is the kind the wizard offers for a script: the one its job
// already declares, so a re-init shows the file rather than re-deriving it from
// the script's name.
func ProposedScriptKind(params ProposedScriptKindParams) domain.JobKind {
	cmd := ScriptJobCmd(params.PackageManager, params.Script.Name)
	cwd := ScriptJobCwd(params.Script.Workspace)
	for _, job := range params.Config.Jobs {
		if job.Cmd == cmd && job.Cwd == cwd && job.Kind != "" {
			return job.Kind
		}
	}
	return ClassifyScriptKind(params.Script.Name)
}
