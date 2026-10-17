package output

import (
	"encoding/json"
	"io"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// Envelope writes the v2 JSON document of doc to w, indented, each field it refuses spelled by spell; it returns the error of the write.
func Envelope(w io.Writer, doc domain.Envelope, spell domain.Spell) error {
	raw, err := doc.JSON(spell)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(json.RawMessage(raw))
}
