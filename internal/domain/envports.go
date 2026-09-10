package domain

// EnvPortLink is one [[env_port]] entry of run.toml: the .env key whose value
// carries the host port of a declared job port. The link names the key only —
// where the port sits inside that value is found by looking for the declared
// base, so a bare port and a port buried in a URL are the same case.
type EnvPortLink struct {
	File string `toml:"file" json:"file"`
	Key  string `toml:"key"  json:"key"`
	// Job names which job's port the key follows. Two jobs may each declare a
	// PORT — their environments are separate — so the port name alone does not
	// identify a base. Every port belongs to exactly one job, so this is always
	// knowable and always required.
	Job  string `toml:"job"  json:"job"`
	Port string `toml:"port" json:"port"`
	// ByDir says the link was attached from the directory holding the file
	// rather than from a value carrying the declared base. It never reaches
	// run.toml — it only marks the proposal, so the reader can tell a match
	// from a deduction before confirming.
	ByDir bool `toml:"-" json:"-"`
}

// EnvValueLink is a .env key whose whole value wtm writes, from a template over
// what only wtm knows: the slice of a shared service this worktree holds, the
// ports it binds, the address it answers on. It is the counterpart of
// EnvPortLink and not a wider spelling of it — a port link substitutes the port
// inside a value it otherwise leaves alone, which is what lets a password never
// be spelled in run.toml; this one owns the line.
type EnvValueLink struct {
	File string `toml:"file" json:"file"`
	Key  string `toml:"key"  json:"key"`
	// Job names the service the value speaks about: whose namespace {namespace}
	// resolves to, whose ports {port.NAME} reads, whose published address
	// {origin} is.
	Job string `toml:"job" json:"job"`
	// Value is the template. See domain.EnvValueToken* for the vocabulary; a
	// placeholder outside it is refused at load rather than written to a file.
	Value string `toml:"value" json:"value"`
}

// EnvKeyRef is one .env key in one file — the pair every table writing a value
// competes for, and the identity a step's row is remembered by.
type EnvKeyRef struct {
	File string
	Key  string
}

// EnvValueField is one candidate row of the [[env]] step: a .env key that could
// follow a shared service's slice. wtm cannot detect which key is a realm or a
// database name — the value is opaque, with none of the three signs that make a
// port recognizable — so every managed key is offered and the reader points.
type EnvValueField struct {
	File string
	Key  string
	// Job is the shared service this row would follow.
	Job string
	// Current is what the key holds today, shown beside the row: it is the only
	// thing that lets a reader recognize which key is the one they mean.
	Current string
	// Value is the template written when Linked. It starts at {namespace}, the
	// one proposal wtm can make honestly.
	Value  string
	Linked bool
	// Vars is the vocabulary, shown while the template is being edited.
	Vars []NamespaceVarGroup
}

// PortKeyWrite is one declared port materialized as a .env key: the base goes
// into the value file and into its committed template, and the [[env_port]]
// link makes each worktree's offset follow.
type PortKeyWrite struct {
	Job      string `json:"job"`
	Port     string `json:"port"`
	Base     int    `json:"base"`
	File     string `json:"file"`
	Template string `json:"template"`
	// AddTarget says nothing provisions that file yet, so the [env] target has
	// to be written too — without it a worktree would not have the file at all.
	AddTarget bool `json:"add_target"`
}

// PortRef identifies one declared port: the job that carries it and its name.
type PortRef struct {
	Job  string
	Name string
}

// EnvPortStatus is the verdict for one link against the value the .env holds.
// Only EnvPortStatusRewrite is ever written back: the rest are reported, because
// guessing which number of a URL is the port can corrupt it.
type EnvPortStatus string

const (
	// EnvPortStatusRewrite means the base was found exactly once and the value changes.
	EnvPortStatusRewrite EnvPortStatus = "rewrite"
	// EnvPortStatusUnchanged means the value already holds the resolved port — the
	// main worktree, whose offset is zero, or a file reconciled twice.
	EnvPortStatusUnchanged EnvPortStatus = "unchanged"
	// EnvPortStatusMissingKey means the .env has no such key.
	EnvPortStatusMissingKey EnvPortStatus = "missing_key"
	// EnvPortStatusNotFound means neither the base nor the resolved port appears in
	// the value; nothing anchors the substitution.
	EnvPortStatusNotFound EnvPortStatus = "base_not_found"
	// EnvPortStatusAmbiguous means the port appears more than once in the value.
	// It cannot arise on an origin rewrite: replacing an authority is structural,
	// so a port sitting in a path or a query is never a candidate.
	EnvPortStatusAmbiguous EnvPortStatus = "ambiguous"
	// EnvPortStatusForeignHost means the value is a URL pointing somewhere the
	// proxy does not serve — a staging host, say. The link names a local job, so
	// the two disagree and wtm reports rather than picks.
	EnvPortStatusForeignHost EnvPortStatus = "foreign_host"
	// EnvPortStatusSecureScheme means the value is https and the proxy speaks
	// plain HTTP. Downgrading a scheme the user wrote on purpose is not wtm's
	// call, and an app serving https locally already has its own CA.
	EnvPortStatusSecureScheme EnvPortStatus = "secure_scheme"
)

// EnvPortMove is one port a key follows: what it is declared at, and what this
// worktree binds it to.
type EnvPortMove struct {
	// Port names the declaration the move follows, so a key following two jobs
	// says which is which.
	Port     string `json:"port"`
	Job      string `json:"job"`
	Base     int    `json:"base"`
	Resolved int    `json:"resolved"`
}

// EnvPortEntry is one link resolved against the worktree's offset and the value
// currently in the file. NewValue is meaningful only for EnvPortStatusRewrite.
type EnvPortEntry struct {
	File string `json:"file"`
	Key  string `json:"key"`
	Port string `json:"port"`
	// Base and Resolved are the first port the key follows. A value holding a
	// list of origins follows several — Moves carries them all, this pair its
	// first, which is every reading written before a key could follow more than
	// one.
	Base     int           `json:"base"`
	Resolved int           `json:"resolved"`
	Moves    []EnvPortMove `json:"moves,omitempty"`
	// Addressing is how this one entry was resolved, not what the project asked
	// for: a project on AddressingNames still resolves its bare-port links by
	// port, and the table has to render the two differently.
	Addressing   Addressing    `json:"addressing"`
	Status       EnvPortStatus `json:"status"`
	CurrentValue string        `json:"current_value,omitempty"`
	NewValue     string        `json:"new_value,omitempty"`
	// ForeignHost is what the value pointed at when it pointed somewhere the
	// proxy does not serve. Only EnvPortStatusForeignHost carries it.
	ForeignHost string `json:"foreign_host,omitempty"`
}

// EnvPortPlan is every link of a worktree resolved at once, entries ordered as
// run.toml declares them.
type EnvPortPlan struct {
	Offset  int            `json:"offset"`
	Entries []EnvPortEntry `json:"entries"`
	// Addressing is what the project asked for, which is not always what the
	// entries got: a machine with no proxy writes ports whatever run.toml says.
	Addressing Addressing `json:"addressing"`
	// PublicPort is what an address announces here, zero when nothing serves
	// names. Both are what the notices are derived from.
	PublicPort int `json:"public_port,omitempty"`
	// Owned is the worktree identity the .env carries: keys wtm derives rather
	// than reconciles, kept apart from Entries because they follow no port.
	Owned []EnvOwnedEntry `json:"owned,omitempty"`
	// Applied says the rewrites were written. A plan that was only computed — a
	// --check run, or one the user declined — carries them all the same, and
	// counting those as written would be a false report.
	Applied bool `json:"applied"`
}

// RunAddresses is where each worktree's jobs answer, plus what has to be said
// about the answer: a worktree whose .env was never settled on the names it
// publishes is served its ports, and Notes is the line saying so.
type RunAddresses struct {
	ByBranch map[string]map[string]JobAddress
	Notes    map[string]string
	// PortAddressed keys the worktrees served their ports rather than the names
	// they publish. It is carried out with the addresses because a surface that
	// opens a board of its own has to reach the same verdict: deriving it from
	// Notes would not, since a worktree can drift enough to warrant a line
	// without being addressed by port.
	PortAddressed map[string]bool
}
