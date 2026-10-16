package project

import "slices"

type EnvStrategy string

const (
	EnvExample EnvStrategy = "example"
	EnvMain    EnvStrategy = "main"
	EnvParent  EnvStrategy = "parent"
)

const DefaultEnvStrategy = EnvExample

func EnvStrategies() []EnvStrategy {
	return []EnvStrategy{EnvExample, EnvMain, EnvParent}
}

func (s EnvStrategy) Valid() bool {
	return slices.Contains(EnvStrategies(), s)
}
