package runenv

import "github.com/LucasPcq/wtm/next/internal/wtm/dotenv"

type HookEnvParams struct {
	Params
	ComposeEnv [][]dotenv.Line
}

func HookEnv(params HookEnvParams) map[string]string {
	env := JobEnv(params.Params)
	if name := declaredComposeProject(params.ComposeEnv); name != "" {
		env[envComposeProject] = name
	}
	return env
}
