package domain

// VersionReport is what a host reads before relying on wtm: the binary's
// version, then one key per versioned contract. A reader ignores the keys it
// does not know, so a new contract is a new key, never a breaking change.
type VersionReport struct {
	Version string `json:"version"`
	Events  int    `json:"events"`
}
