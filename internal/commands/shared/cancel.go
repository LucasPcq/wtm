package shared

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
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
