package form

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// A Stream is the form of a command that streams rather than returns, never ending on its own (wtm events): its flags are its schema's, worded by Words, and --output, its request is built from them, and it asks nothing. Its cli prints each line itself, never framed.
type Stream[R Request[R], Q Questions] struct {
	Words Worder[Q]
}

func (s Stream[R, Q]) schema() []worded {
	return wordedSchema(s.Words)
}

// Flags spells each field path of the request as the flag that sets it, for the failures.
func (s Stream[R, Q]) Flags() output.Flags {
	flags, _ := flagsOf(s.schema())
	return flags
}

// Bind registers on cmd a flag per field of the schema, by its kind, then --output, read into run.
func (s Stream[R, Q]) Bind(cmd *cobra.Command, run *uicli.Run) {
	bindFields(cmd, s.schema())
	cmd.Flags().StringVar(&run.Output, "output", uicli.OutputText, uicli.OutputUsage)
}

// Request returns the request cmd's flags and args give, asked from where env stands.
func (s Stream[R, Q]) Request(cmd *cobra.Command, args []string, env uicli.Env) R {
	return domain.CalledBy(folded[R](fieldAnswers(cmd, args, s.schema())), env.Caller())
}
