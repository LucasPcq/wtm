package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// strictParams is what decodeStrict decodes: Data, read from Source (named in
// errors), into Out; keys under a table of Ignore are left to the caller.
type strictParams struct {
	Source string
	Data   []byte
	Out    any
	Ignore []string
}

// decodeStrict decodes params.Data into params.Out and refuses any key Out has
// no field for, outside the ignored tables, so a typo in a config file is an
// error rather than a setting silently dropped.
func decodeStrict(params strictParams) error {
	meta, err := toml.Decode(string(params.Data), params.Out)
	if err != nil {
		return fmt.Errorf("parse %s: %w", params.Source, err)
	}

	var unknown []string
	for _, key := range meta.Undecoded() {
		name := key.String()
		ignored := slices.ContainsFunc(params.Ignore, func(prefix string) bool { return name == prefix || strings.HasPrefix(name, prefix+".") })
		if !ignored {
			unknown = append(unknown, name)
		}
	}

	if len(unknown) > 0 {
		return fmt.Errorf("unknown keys in %s: %s", params.Source, strings.Join(unknown, ", "))
	}
	return nil
}
