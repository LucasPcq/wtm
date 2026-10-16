package statefile

import "github.com/LucasPcq/wtm/next/internal/wtm"

type Schema struct {
	Path      string
	Found     int
	Supported int
}

func Newer(schema Schema) bool {
	return max(schema.Found, 1) > schema.Supported
}

func Stamp(supported int) int {
	if supported <= 1 {
		return 0
	}
	return supported
}

func Refusal(schema Schema) error {
	if !Newer(schema) {
		return nil
	}
	return &wtm.SchemaNewer{Path: schema.Path, Found: schema.Found, Supported: schema.Supported}
}
