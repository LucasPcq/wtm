package domain

// ShellType represents a supported shell for integration.
type ShellType string

const (
	ShellZsh  ShellType = "zsh"
	ShellBash ShellType = "bash"
	ShellFish ShellType = "fish"
)

// ProjectConfig maps to .wtm.toml (project-level configuration).
type ProjectConfig struct {
	Worktrees WorktreesConfig `toml:"worktrees" json:"worktrees"`
	Env       EnvConfig       `toml:"env" json:"env"`
	Hooks     HooksConfig     `toml:"hooks" json:"hooks"`
}

// WorktreesConfig controls worktree creation defaults.
type WorktreesConfig struct {
	BasePath   string `toml:"base_path" json:"base_path"`
	BaseBranch string `toml:"base_branch" json:"base_branch"`
}

// EnvFile is a detected/configured env value file and its committed template.
// Target is the value file to provision (e.g. ".env", "apps/api/.env",
// ".env.local"); Template is the committed schema it derives from (e.g.
// ".env.example"), empty when the file is value-only. Local marks a machine-local
// override (".env.local") for informational display — it stays syncable.
type EnvFile struct {
	Target   string `toml:"target" json:"target"`
	Template string `toml:"template" json:"template"`
	Local    bool   `toml:"local" json:"local"`
}

// EnvConfig controls .env file provisioning. Files is the single source of truth
// for the value targets and their committed templates.
type EnvConfig struct {
	Strategy EnvStrategy `toml:"strategy" json:"strategy"`
	Files    []EnvFile   `toml:"file" json:"files"`
}

// HooksConfig defines lifecycle hooks as lists of commands.
type HooksConfig struct {
	OnCreate []HookCommand `toml:"on_create" json:"on_create"`
	OnClean  []HookCommand `toml:"on_clean" json:"on_clean"`
}

// GlobalConfig maps to the global wtm config (user-level configuration).
type GlobalConfig struct {
	Shell  ShellType    `toml:"shell"`
	UI     UIConfig     `toml:"ui"`
	Proxy  ProxyConfig  `toml:"proxy"`
	Update UpdateConfig `toml:"update"`
}

// UpdateConfig groups update-check preferences. Check is a pointer so an absent
// key is distinguishable from an explicit false — see rules.ShouldCheckUpdate.
type UpdateConfig struct {
	Check *bool `toml:"check" json:"check"`
}

// UIConfig groups display preferences. Animations is a pointer so an absent
// key ("nothing wrote here") is distinguishable from an explicit false — see
// rules.AnimationsEnabled.
type UIConfig struct {
	Animations *bool `toml:"animations" json:"animations"`
}

// Config is the merged, validated configuration used by the service layer.
type Config struct {
	Project ProjectConfig
	Global  GlobalConfig
}
