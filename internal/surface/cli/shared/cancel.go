package shared

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

// MarkCancelled records that the user backed out of cmd. A flow concludes an
// abort with a notice and no error, which is right for the dashboard and wrong
// for a shell: the root reads this mark to exit on ExitCodeCancelled, whether
// the runner then returns nil or ErrAborted (its report already on screen).
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

// BackedOut concludes a command whose confirmation the user declined or
// escaped: the `=` line every abort reads as, and the cancelled exit code.
// Any other error is the command's to return.
func BackedOut(cmd *cobra.Command, err error) error {
	if err != nil && !errors.Is(err, domain.ErrUserAborted) {
		return err
	}
	render.Frame(cmd.OutOrStdout(), func(w io.Writer) { render.Unchanged(w, domain.AbortedMessage) })
	MarkCancelled(cmd)
	return nil
}
