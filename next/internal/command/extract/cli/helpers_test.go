package cli

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	update      = flag.Bool("update", false, "rewrite the golden files")
	testdata, _ = filepath.Abs("testdata")
)

type printed struct {
	Stdout string
	Stderr string
	Code   int
}

func runExtract(t *testing.T, w world, words ...string) printed {
	t.Helper()
	streams, _, stdout, stderr := output.Test()
	root := uicli.NewRoot("1.2.3", streams, []*cobra.Command{New(streams, w.open)})
	code := uicli.Execute(t.Context(), root, append([]string{"extract"}, words...))
	return printed{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}

type envelope struct {
	V       int    `json:"v"`
	Command string `json:"command"`
	OK      bool   `json:"ok"`
	Items   []struct {
		Subject string          `json:"subject"`
		Status  string          `json:"status"`
		Detail  json.RawMessage `json:"detail"`
	} `json:"items"`
	Plan *struct {
		Source string `json:"source"`
		Target struct {
			Branch string `json:"branch"`
			Action string `json:"action"`
		} `json:"target"`
		Keep bool `json:"keep"`
	} `json:"plan"`
	Fingerprint string `json:"fingerprint"`
	Error       *struct {
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

func paths(changes []worktree.Change) []string {
	found := make([]string, 0, len(changes))
	for _, change := range changes {
		found = append(found, change.Path)
	}
	return found
}
