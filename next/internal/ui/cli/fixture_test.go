package cli

import "github.com/spf13/cobra"

const codeConflict = "extract.conflict"

func probe() *cobra.Command {
	return &cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return nil }}
}

// envBrokenPipeHelper turns the test binary into a helper that writes to a stdout whose reader left.
const envBrokenPipeHelper = "WTM_NEXT_TEST_BROKEN_PIPE_HELPER"
