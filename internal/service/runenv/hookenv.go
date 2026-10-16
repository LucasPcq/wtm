package runenv

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type HookEnvParams struct {
	Params
	WorktreePath string
}

func HookEnv(params HookEnvParams) (map[string]string, error) {
	env, err := JobEnv(params.Params)
	if err != nil {
		return nil, err
	}
	files, err := composeEnvFiles(composeEnvFilesParams{Params: params.Params, Root: params.WorktreePath})
	if err != nil {
		return nil, err
	}
	if name := rules.DeclaredComposeProject(files); name != "" {
		env[domain.EnvComposeProjectName] = name
	}
	return env, nil
}
