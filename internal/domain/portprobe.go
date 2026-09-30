package domain

// PortProbeStatus is what a probe found on the port a job declared. It says
// what was observed, never what is at fault: wtm can see that nothing answers,
// not why.
type PortProbeStatus string

const (
	PortListening PortProbeStatus = "listening"
	PortSilent    PortProbeStatus = "silent"
)

// PortProbe is one declared port checked after its job started.
type PortProbe struct {
	Job  string `json:"job"`
	Name string `json:"name"`
	// Port is the resolved port, base plus this worktree's offset.
	Port   int             `json:"port"`
	Status PortProbeStatus `json:"status"`
	// BaseListening is the base port when it answers and the resolved one does
	// not — the signature of a variable that never reached the process. Zero
	// otherwise, and always zero on the main checkout, where base and resolved
	// are the same port and there is nothing to tell apart.
	BaseListening int `json:"base_listening,omitempty"`
	// BaseOwner is the worktree bound to that base port, when one is. The main
	// checkout is the only worktree whose offset is zero, so the only one that
	// can hold a base port — and when it does, the silent resolved port says
	// nothing about this job's own command.
	BaseOwner string `json:"base_owner,omitempty"`
}

// PortEntry is one declared port as a surface shows it: the job it belongs to,
// the variable it is injected under, and the base the main checkout binds.
type PortEntry struct {
	Job  string
	Name string
	Base int
	// BindsNone is the row answered "this service listens on nothing". It is a
	// settled answer, unlike a zero Base, which is a question left open.
	BindsNone bool
	// CanBindNone is whether that answer means anything here. A job declaring a
	// port cannot also bind none — run.toml refuses the pair — so its rows take
	// a base and nothing else.
	CanBindNone bool
}
