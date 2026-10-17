package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	update      = flag.Bool("update", false, "rewrite the golden files")
	testdata, _ = filepath.Abs("testdata")
)

type ran struct {
	Stdout string
	Stderr string
	Code   int
}

func runIn(t *testing.T, w world, args ...string) ran {
	t.Helper()
	root := NewRoot(w.params(args))
	var stdout, stderr bytes.Buffer
	root.SetIn(&bytes.Buffer{})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	code := Execute(t.Context(), root)
	return ran{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}

func runCreate(t *testing.T, w world, words ...string) ran {
	t.Helper()
	return runIn(t, w, append([]string{"create"}, words...)...)
}

type envelope struct {
	OK    bool   `json:"ok"`
	Op    string `json:"op"`
	Items []struct {
		Subject string `json:"subject"`
		Status  string `json:"status"`
		Reason  string `json:"reason"`
	} `json:"items"`
	Plan *struct {
		Branches []struct {
			Branch string `json:"branch"`
			Action string `json:"action"`
		} `json:"branches"`
		From        string   `json:"from"`
		EnvFrom     string   `json:"env_from"`
		FastForward []string `json:"fast_forward"`
		Remember    []string `json:"remember"`
	} `json:"plan"`
	Error *struct {
		Kind    string `json:"kind"`
		Code    string `json:"code"`
		Message string `json:"message"`
		Fields  []struct {
			Path string `json:"path"`
			Code string `json:"code"`
			Flag string `json:"flag"`
		} `json:"fields"`
	} `json:"error"`
}

func decoded(t *testing.T, stdout string) envelope {
	t.Helper()
	var doc envelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc), stdout)
	return doc
}

func golden(t *testing.T, name string, got string) {
	t.Helper()
	path := filepath.Join(testdata, name+".golden")
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}
