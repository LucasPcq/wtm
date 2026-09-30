package domain

// DevOriginsFiles are the names a Next project's config goes by, in the order
// they are looked for. A var because a Go const cannot hold a slice.
var DevOriginsFiles = []string{"next.config.js", "next.config.mjs", "next.config.ts"}

// ProxyRoute is one entry of the run proxy's table: a hostname, and the loopback
// address of the job answering under it. It is a projection of a running job,
// never a second source of truth.
type ProxyRoute struct {
	Host     string `json:"host"`
	Target   string `json:"target"`
	Job      string `json:"job"`
	Worktree string `json:"worktree"`
	Project  string `json:"project"`
}

// ProxyConfig is the [proxy] table of the global wtm config. The port is a
// property of the machine, not of a repository: the daemon is global and serves
// every repo at once.
type ProxyConfig struct {
	Port int `toml:"port,omitempty" json:"port,omitempty"`
	// Enabled is a pointer for the same reason UIConfig.Animations is: absent
	// and explicitly false are different answers.
	Enabled *bool `toml:"enabled,omitempty" json:"enabled,omitempty"`
}

// ProxyStatus is what `wtm run proxy status` reports: the configured and real
// bind ports, the declared redirection, and what the probe actually saw.
type ProxyStatus struct {
	Supported      bool   `json:"supported"`
	Mechanism      string `json:"mechanism,omitempty"`
	Installed      bool   `json:"installed"`
	ConfiguredPort int    `json:"configured_port"`
	BindPort       int    `json:"bind_port,omitempty"`
	PublicPort     int    `json:"public_port"`
	Probed         int    `json:"probed_bind_port,omitempty"`
	DaemonUp       bool   `json:"daemon_up"`
	// ConfigPath is where [proxy] is read from, resolved rather than spelled: it
	// is the one place a reader can learn the path, since os.UserConfigDir()
	// differs per platform and no message may hard-code it.
	ConfigPath string `json:"config_path,omitempty"`
	Diverged   bool   `json:"diverged"`
	ExampleURL string `json:"example_url,omitempty"`
}

// ProxyPlannedFile is one file a privileged install would write, rendered
// before anything is written so the recap and the write share one source.
type ProxyPlannedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	// Change is the one-line description the recap shows instead of Content: a
	// reader decides on the three paths, not on Apple's pf.conf boilerplate.
	Change string `json:"change"`
}
