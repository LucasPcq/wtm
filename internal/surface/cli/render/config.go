package output

import (
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ConfigValidateJSON is the payload emitted by `config show --validate --output json`.
type ConfigValidateJSON struct {
	Valid bool   `json:"valid"`
	Error string `json:"error,omitempty"`
}

// WriteProjectConfigJSON writes the structured project config as JSON.
func WriteProjectConfigJSON(w io.Writer, cfg domain.ProjectConfig) error {
	return encodeJSON(w, cfg)
}

// WriteConfigValidateJSON writes the config validation outcome as JSON.
func WriteConfigValidateJSON(w io.Writer, payload ConfigValidateJSON) error {
	return encodeJSON(w, payload)
}
