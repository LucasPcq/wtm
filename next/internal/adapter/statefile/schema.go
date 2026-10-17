package statefile

import "github.com/LucasPcq/wtm/next/internal/wtm"

type Schema struct {
	Path      string
	Found     int
	Supported int
}

func Refusal(schema Schema) error {
	if max(schema.Found, 1) <= schema.Supported {
		return nil
	}
	return &wtm.SchemaNewer{Path: schema.Path, Found: schema.Found, Supported: schema.Supported}
}
