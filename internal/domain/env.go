package domain

// EnvLineKind classifies a logical line of a .env document.
type EnvLineKind string

const (
	EnvLinePair    EnvLineKind = "pair"
	EnvLineComment EnvLineKind = "comment"
	EnvLineBlank   EnvLineKind = "blank"
)

// EnvLine is one logical line of a .env document. Raw holds the exact original
// text (newline excluded) and is what gets rendered: a value is changed through
// rules.WithEnvValue, which keeps Raw in step. Key, Value and Export are
// meaningful only for EnvLinePair; Value is opaque (inner text, unquoted, no
// expansion or escape interpretation).
type EnvLine struct {
	Kind   EnvLineKind
	Key    string
	Value  string
	Export bool
	Raw    string
}

// EnvPortScan is what one directory's env files say about the ports of the job
// running there. SourceByVar names the file each port was read from — the recap
// shows it so the user can go check the value wtm picked up.
type EnvPortScan struct {
	Dir         string
	Err         string
	Ports       map[string]int
	SourceByVar map[string]string
}

// EnvOwnedEntry is one wtm-owned key as it lands in a worktree's .env. Changed
// says the file did not already hold that value.
type EnvOwnedEntry struct {
	File    string `json:"file"`
	Key     string `json:"key"`
	Value   string `json:"value"`
	Changed bool   `json:"changed"`
}
