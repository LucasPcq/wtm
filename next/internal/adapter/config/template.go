package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

//go:embed config.toml.tmpl
var configTemplate string

var parsedTemplate = template.Must(template.New("config.toml").Funcs(template.FuncMap{"toml": tomlString, "tomlKey": tomlKey}).Parse(configTemplate))

// templateData is what config.toml.tmpl renders.
type templateData struct {
	BasePath    string
	BaseBranch  string
	SkipEnv     bool
	EnvStrategy string
	EnvFiles    []project.EnvFile
	OnCreate    []project.Hook
	OnClean     []project.Hook
	Remembered  map[string]string
}

// render returns config.toml rendered from saved with its template, comments
// included, the way wtm init writes it.
func render(saved savedProject) ([]byte, error) {
	config := saved.Config
	data := templateData{
		BasePath:    config.Worktrees.BasePath,
		BaseBranch:  config.Worktrees.BaseBranch,
		SkipEnv:     config.Env.Strategy == "" && len(config.Env.Files) == 0,
		EnvStrategy: string(config.Env.Strategy),
		EnvFiles:    config.Env.Files,
		OnCreate:    config.OnCreate,
		OnClean:     saved.OnClean,
		Remembered:  config.Remembered,
	}

	var buf bytes.Buffer
	if err := parsedTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render template: %w", err)
	}

	return buf.Bytes(), nil
}

// tomlString returns value as a TOML string literal, quoted and escaped.
func tomlString(value string) (string, error) {
	line, err := encodedLine(map[string]string{"v": value})
	return strings.TrimPrefix(line, "v = "), err
}

// tomlKey returns key as a TOML key, quoted only when it must be.
func tomlKey(key string) (string, error) {
	line, err := encodedLine(map[string]string{key: ""})
	return strings.TrimSuffix(line, ` = ""`), err
}

// encodedLine returns the one line the TOML encoder writes for pair.
func encodedLine(pair map[string]string) (string, error) {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(pair); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}
