package form_test

import (
	"testing"

	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAStreamBindsItsFieldsAndOutputAndReadsItsRequestFromThem(t *testing.T) {
	var stream form.Stream[request, questions]
	run := &uicli.Run{}
	cmd := &cobra.Command{Use: "stream"}
	stream.Bind(cmd, run)
	types := map[string]string{}
	cmd.Flags().VisitAll(func(f *pflag.Flag) { types[f.Name] = f.Value.Type() })

	require.NoError(t, cmd.Flags().Parse([]string{"--name", "x", "--ff", "--output", "json"}))

	assert.Equal(t, map[string]string{
		"from": "string", "mode": "string", "ff": "bool", "keep": "bool", "files": "stringSlice", "name": "string", "output": "string",
	}, types)
	assert.Equal(t, request{"name": {"x"}, "ff": {"true"}}, stream.Request(cmd, nil))
	assert.Equal(t, uicli.OutputJSON, run.Output)
	assert.Equal(t, "--name", stream.Flags().Spell("name"))
}
