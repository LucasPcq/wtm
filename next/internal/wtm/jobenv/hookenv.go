package jobenv

import (
	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

type HookEnvParams struct {
	Params
	ComposeEnv [][]dotenv.Line
}

func HookEnv(params HookEnvParams) map[string]string {
	env := JobEnv(params.Params)
	if name := declaredComposeProject(params.ComposeEnv); name != "" {
		env[ComposeProjectKey] = name
	}
	return env
}

type HooksReadRunParams struct {
	Run       run.Config
	Isolation run.Isolation
}

func HooksReadRun(params HooksReadRunParams) bool {
	return params.Isolation != "" && len(ComposeDirs(params.Run)) > 0
}
