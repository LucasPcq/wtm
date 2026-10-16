package wtm

const (
	DefaultBasePath   = "../.trees"
	DefaultBaseBranch = "main"
)

type ProjectConfig struct {
	Schema    int             `toml:"schema,omitempty"`
	Worktrees WorktreesConfig `toml:"worktrees"`
	Env       EnvConfig       `toml:"env"`
	Hooks     HooksConfig     `toml:"hooks"`
	Wizard    WizardConfig    `toml:"wizard"`
}

type WorktreesConfig struct {
	BasePath   string `toml:"base_path"`
	BaseBranch string `toml:"base_branch"`
}

type EnvConfig struct {
	Strategy EnvStrategy `toml:"strategy"`
	Files    []EnvFile   `toml:"file"`
}

type EnvFile struct {
	Target   string `toml:"target"`
	Template string `toml:"template"`
	Local    bool   `toml:"local"`
}

type HooksConfig struct {
	OnCreate []HookCommand `toml:"on_create"`
	OnClean  []HookCommand `toml:"on_clean"`
}

type HookCommand struct {
	Cmd             string `toml:"cmd"`
	Cwd             string `toml:"cwd"`
	ContinueOnError bool   `toml:"continue_on_error"`
}

type WizardConfig struct {
	Remembered map[string]string `toml:"remembered"`
}

type GlobalConfig struct {
	Schema int          `toml:"schema"`
	Shell  string       `toml:"shell"`
	UI     UIConfig     `toml:"ui"`
	Proxy  ProxyConfig  `toml:"proxy"`
	Update UpdateConfig `toml:"update"`
}

type UIConfig struct {
	Animations *bool `toml:"animations"`
}

type ProxyConfig struct {
	Port    int   `toml:"port,omitempty"`
	Enabled *bool `toml:"enabled,omitempty"`
}

type UpdateConfig struct {
	Check *bool `toml:"check"`
}
