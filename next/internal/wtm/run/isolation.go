package run

import "slices"

type Isolation string

const (
	Isolated Isolation = "isolated"
	Verbatim Isolation = "verbatim"
)

func Isolations() []Isolation {
	return []Isolation{Isolated, Verbatim}
}

func (i Isolation) Valid() bool {
	return slices.Contains(Isolations(), i)
}

func (i Isolation) Effective() Isolation {
	if i == "" {
		return Isolated
	}
	return i
}

func (c Config) IsolationApplies() bool {
	isolates := func(job Job) bool { return len(job.Ports) > 0 || job.HasNamespace() || job.RunsCompose() }
	return len(c.EnvPorts) > 0 || len(c.EnvValues) > 0 || slices.ContainsFunc(c.Jobs, isolates)
}
