package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sample struct {
	Name  string         `toml:"name"`
	Port  int            `toml:"port"`
	Hooks map[string]any `toml:"hooks"`
}

func TestAValidFileDecodes(t *testing.T) {
	var decoded sample

	err := decodeStrict(strictParams{Source: "test", Data: []byte("name = \"api\"\nport = 8080\n"), Out: &decoded})

	require.NoError(t, err)
	assert.Equal(t, sample{Name: "api", Port: 8080}, decoded)
}

func TestAnUnknownKeyIsRefusedByName(t *testing.T) {
	var decoded sample

	err := decodeStrict(strictParams{Source: "test", Data: []byte("name = \"api\"\nunknown = 42\n"), Out: &decoded})

	assert.EqualError(t, err, "unknown keys in test: unknown")
}

func TestAnUnknownSectionIsRefusedByName(t *testing.T) {
	var decoded sample

	err := decodeStrict(strictParams{Source: "test", Data: []byte("name = \"api\"\n\n[extras]\ncolor = \"red\"\n"), Out: &decoded})

	assert.ErrorContains(t, err, "extras")
}

func TestAParseErrorIsReported(t *testing.T) {
	var decoded sample

	err := decodeStrict(strictParams{Source: "test", Data: []byte("not = valid = toml"), Out: &decoded})

	assert.ErrorContains(t, err, "parse test:")
}

func TestAnIgnoredSubtreeIsNotChecked(t *testing.T) {
	var decoded sample

	err := decodeStrict(strictParams{Source: "test", Data: []byte("[hooks]\non_create = [{ cmd = \"x\", odd = 1 }]\n"), Out: &decoded, Ignore: []string{"hooks.on_create"}})

	assert.NoError(t, err)
}
