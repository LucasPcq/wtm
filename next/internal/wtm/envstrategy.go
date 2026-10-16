package wtm

type EnvStrategy string

const (
	EnvStrategyExample EnvStrategy = "example"
	EnvStrategyMain    EnvStrategy = "main"
	EnvStrategyParent  EnvStrategy = "parent"
)

const DefaultEnvStrategy = EnvStrategyExample

var EnvStrategies = []EnvStrategy{EnvStrategyExample, EnvStrategyMain, EnvStrategyParent}

func (s EnvStrategy) Valid() bool {
	return s == EnvStrategyExample || s == EnvStrategyMain || s == EnvStrategyParent
}
