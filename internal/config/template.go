package config

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
)

//go:embed wtm.toml.tmpl
var projectConfigTemplate string

var parsedTemplate = template.Must(template.New("wtm.toml").
	Funcs(template.FuncMap{"toml": tomlString, "tomlKey": tomlKey}).
	Parse(projectConfigTemplate))

// tomlString and tomlKey take their spelling from the encoder of the parser that
// reads the file back, so any value round-trips: quotes, backslashes, newlines.
func tomlString(value string) (string, error) {
	line, err := encodedLine(map[string]string{"v": value})
	return strings.TrimPrefix(line, "v = "), err
}

func tomlKey(key string) (string, error) {
	line, err := encodedLine(map[string]string{key: ""})
	return strings.TrimSuffix(line, ` = ""`), err
}

func encodedLine(pair map[string]string) (string, error) {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(pair); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}
