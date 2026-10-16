package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

type strictParams struct {
	Source string
	Data   []byte
	Out    any
	Ignore []string
}

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
