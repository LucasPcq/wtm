package run

type Isolation string

const (
	Isolated Isolation = "isolated"
	Verbatim Isolation = "verbatim"
)

func Isolations() []Isolation {
	return []Isolation{Isolated, Verbatim}
}

func (i Isolation) Valid() bool {
	return i == Isolated || i == Verbatim
}

func (i Isolation) Effective() Isolation {
	if i == "" {
		return Isolated
	}
	return i
}

func (c Config) IsolationApplies() bool {
	if len(c.EnvPorts) > 0 || len(c.EnvValues) > 0 {
		return true
	}
	for _, job := range c.Jobs {
		if len(job.Ports) > 0 || job.HasNamespace() || job.RunsCompose() {
			return true
		}
	}
	return false
}
