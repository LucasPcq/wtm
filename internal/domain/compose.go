package domain

// ComposePortStatus says what a compose port mapping allows wtm to do with it.
// Only a frozen one is ever rewritten: a templated mapping already reads a
// variable, and an unsupported one wtm refuses to touch.
type ComposePortStatus string

const (
	ComposePortTemplated   ComposePortStatus = "templated"
	ComposePortFrozen      ComposePortStatus = "frozen"
	ComposePortUnsupported ComposePortStatus = "unsupported"
)

// ComposePortBinding is one entry of a service's `ports:` list, located in the
// source file precisely enough to be rewritten without re-serializing it.
type ComposePortBinding struct {
	File    string
	Service string
	Status  ComposePortStatus
	Reason  string
	Var     string
	// Base is the host port the main checkout binds; Container never shifts.
	Base      int
	Container int
	// Line and Column are 1-based, Column pointing at the scalar's first
	// character, opening quote included.
	Line   int
	Column int
	// Token is the scalar exactly as the file spells it, quotes included, and
	// Replacement what it becomes — empty unless Status is ComposePortFrozen.
	Token       string
	Replacement string
}

// ComposeScan is everything one docker-compose file says about what a second
// worktree would collide on: its ports, and the names it pins absolutely. Err
// is set when the file could not be read or parsed at all, and the two lists
// are then empty: the file is reported rather than used.
type ComposeScan struct {
	File     string
	Err      string
	Bindings []ComposePortBinding
	Names    []ComposeAbsoluteName
	// Services is what the file declares, in declaration order. It is what the
	// scope step enumerates: wtm generates one job per compose file, so a
	// service is only a candidate for sharing once it has been named here.
	Services []ComposeService
}

// ComposeService is one entry under `services:`. Image and HasBuild are what
// separate a candidate for sharing from a service that cannot be one: a service
// built from this worktree's source serves this worktree's code, whatever its
// name suggests.
type ComposeService struct {
	Name     string
	Image    string
	HasBuild bool
}

// ComposeNameKind names which absolute identifier a finding pins. All three
// are resolved by the Docker daemon, not by the compose project, so they
// ignore COMPOSE_PROJECT_NAME entirely — which is what makes two worktrees
// running the same file collide on them.
type ComposeNameKind string

const (
	ComposeNameContainer ComposeNameKind = "container"
	ComposeNameVolume    ComposeNameKind = "volume"
	ComposeNameNetwork   ComposeNameKind = "network"
)

// ComposeNameStatus says what wtm may do with an absolute name.
type ComposeNameStatus string

const (
	ComposeNameTemplated   ComposeNameStatus = "templated"
	ComposeNameAbsolute    ComposeNameStatus = "absolute"
	ComposeNameUnsupported ComposeNameStatus = "unsupported"
)

// ComposeAbsoluteName is one `container_name:` — or one `name:` under a
// top-level volume or network — located precisely enough to be rewritten
// without re-serializing the file.
type ComposeAbsoluteName struct {
	File string
	Kind ComposeNameKind
	// Owner is the service, volume or network key the name belongs to.
	Owner  string
	Status ComposeNameStatus
	Reason string
	// Name is the value as the file spells it, quotes stripped.
	Name string
	// Line and Column are 1-based, Column pointing at the scalar's first
	// character, opening quote included.
	Line   int
	Column int
	// Token is the scalar exactly as the file spells it, quotes included, and
	// Replacement what it becomes — empty unless Status is ComposeNameAbsolute.
	Token       string
	Replacement string
}

// ComposeEdit is one in-place rewrite of a compose file: the scalar exactly as
// the file spells it, where the scan found it, and what it becomes. Ports and
// absolute names both reduce to this, so a single splice serves the two.
type ComposeEdit struct {
	File        string
	Line        int
	Column      int
	Token       string
	Replacement string
}

// SharedComposeService is one service lifted out of its file's job into a job
// of its own, because it is to run once for the repository. The file's job then
// names the services that stay, since `docker compose up` otherwise starts the
// whole file — including the one just lifted out.
type SharedComposeService struct {
	File    string
	Service string
	Tenant  *JobTenantConfig
}
