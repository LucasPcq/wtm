package shared

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
)

// MarkCancelled records that the user backed out of cmd. A flow concludes an
// abort with a notice and no error, which is right for the dashboard and wrong
// for a shell: the root reads this mark to exit on ExitCodeCancelled.
func MarkCancelled(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[domain.AnnotationCancelled] = domain.AnnotationOn
}

func ClearCancelled(cmd *cobra.Command) {
	delete(cmd.Annotations, domain.AnnotationCancelled)
}

func Cancelled(cmd *cobra.Command) bool {
	return cmd.Annotations[domain.AnnotationCancelled] == domain.AnnotationOn
}

// BackedOut is how a runner ends a run the user backed out of: marked, and
// with no error. An error would skip the root's post-run, which is what reads
// the mark, and the run would exit 1 instead of ExitCodeCancelled.
func BackedOut(cmd *cobra.Command) error {
	MarkCancelled(cmd)
	return nil
}

// EndAborted ends a runner whose flow reported an aborted outcome. The flow's
// abort notice is what marks a user backing out; an outcome aborted without
// it is a run that failed (`run up`'s sequence stopping on a job), which keeps
// its error and its exit code.
func EndAborted(cmd *cobra.Command) error {
	if Cancelled(cmd) {
		return nil
	}
	return domain.ErrAborted
}

// Declined concludes a command whose confirmation the user declined or
// escaped: the `=` line every abort reads as, and the cancelled exit code.
// Any other error is the command's to return.
func Declined(cmd *cobra.Command, err error) error {
	if err != nil && !errors.Is(err, domain.ErrUserAborted) {
		return err
	}
	output.Frame(cmd.OutOrStdout(), func(w io.Writer) { output.Unchanged(w, domain.AbortedMessage) })
	MarkCancelled(cmd)
	return nil
}
