package cli

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const codeConflict = "extract.conflict"

func probe() *cobra.Command {
	return &cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return nil }}
}

// refusing refuses as a repository with no wtm config does, wtm init its follow-up.
func refusing(streams *output.Streams) *cobra.Command {
	run := &Run{Command: "refuse", Streams: streams, Env: Env{Cwd: "/work"}, Output: OutputText, CommandLine: func(followUp domain.FollowUp) []string { return []string{"wtm", followUp.Command} }}
	return &cobra.Command{Use: "refuse", RunE: func(cmd *cobra.Command, _ []string) error {
		if err := run.Begin(cmd); err != nil {
			return err
		}
		return run.FailWith(&domain.Error{Kind: domain.KindPrecondition, Code: "refuse" + domain.ConfigNotFound, Message: "no wtm config found", FollowUp: &domain.FollowUp{Command: domain.CommandInit}}, nil)
	}}
}

// lost runs where its working directory could not be read, in env.
func lost(streams *output.Streams, name string, env Env) *cobra.Command {
	run := &Run{Command: name, Streams: streams, Env: env, Output: OutputText}
	return &cobra.Command{Use: name, RunE: func(cmd *cobra.Command, _ []string) error {
		return run.Begin(cmd)
	}}
}

// envBrokenPipeHelper turns the test binary into a helper that writes to a stdout whose reader left.
const envBrokenPipeHelper = "WTM_NEXT_TEST_BROKEN_PIPE_HELPER"

// envHangupHelper turns the test binary into a helper that waits for its run to be cancelled.
const envHangupHelper = "WTM_NEXT_TEST_HANGUP_HELPER"
