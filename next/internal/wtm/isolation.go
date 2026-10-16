package wtm

type Isolation string

const (
	IsolationIsolated Isolation = "isolated"
	IsolationVerbatim Isolation = "verbatim"
)

var Isolations = []Isolation{IsolationIsolated, IsolationVerbatim}

func (i Isolation) Valid() bool {
	return i == IsolationIsolated || i == IsolationVerbatim
}

func EffectiveIsolation(i Isolation) Isolation {
	if i == "" {
		return IsolationIsolated
	}
	return i
}

func IsolationApplies(run RunConfig) bool {
	if len(run.EnvPorts) > 0 || len(run.EnvValues) > 0 {
		return true
	}
	for _, job := range run.Jobs {
		if len(job.Ports) > 0 || job.HasNamespace() || job.RunsCompose() {
			return true
		}
	}
	return false
}
