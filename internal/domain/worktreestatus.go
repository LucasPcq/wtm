package domain

// StatusProblemCode names an anomaly `wtm status` reports. The codes are a
// contract: a reader keys on them, so one is never renamed.
type StatusProblemCode string

const (
	StatusProblemEnvMissing       StatusProblemCode = "env_missing"
	StatusProblemJobCrashed       StatusProblemCode = "job_crashed"
	StatusProblemIsolationPending StatusProblemCode = "isolation_pending"
)

// StatusProblemCodes is every code, in the order the guide documents them.
var StatusProblemCodes = []StatusProblemCode{
	StatusProblemEnvMissing,
	StatusProblemJobCrashed,
	StatusProblemIsolationPending,
}

// StatusProblem is one anomaly and the command that clears it.
type StatusProblem struct {
	Code    StatusProblemCode `json:"code"`
	Message string            `json:"message"`
	Fix     string            `json:"fix"`
}

// StatusEnv counts the .env files config.toml declares, and names the ones the
// worktree lacks. It never carries a value.
type StatusEnv struct {
	Declared int      `json:"declared"`
	Missing  []string `json:"missing"`
}

// StatusDocument is a worktree's whole state, as `wtm status` reports it.
type StatusDocument struct {
	Branch    string    `json:"branch"`
	Path      string    `json:"path"`
	Main      bool      `json:"main"`
	Isolation Isolation `json:"isolation"`
	// Addressing and Offset are nil without run.toml, and Offset is nil too
	// for a worktree no run has numbered yet.
	Addressing *Addressing     `json:"addressing"`
	Offset     *int            `json:"offset"`
	RunConfig  bool            `json:"run_config"`
	Env        StatusEnv       `json:"env"`
	Jobs       []JobSnapshot   `json:"jobs"`
	Problems   []StatusProblem `json:"problems"`
}

// EnvMissingFile is a declared .env file a worktree lacks, and what there is to
// rebuild it from.
type EnvMissingFile struct {
	Target string
	// Scaffolded says `wtm env` rebuilds it under the worktree's strategy.
	Scaffolded bool
	// HasTemplate says `--from example` would, the strategy's source having no
	// copy.
	HasTemplate bool
}
