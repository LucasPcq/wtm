package cmd

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// usageError keeps cobra's own message and adds domain.ErrUsage to the chain,
// which is what rules.ExitCode reads.
type usageError struct{ err error }

func (e usageError) Error() string   { return e.err.Error() }
func (e usageError) Unwrap() []error { return []error{e.err, domain.ErrUsage} }

func asUsage(err error) error {
	if err == nil {
		return nil
	}
	return usageError{err: err}
}

// markUsageErrors routes every refusal cobra makes on its own — a flag, a value
// or an argument count — through ErrUsage. The root's Args is set because cobra
// only reports an unknown top-level command when it has none.
func markUsageErrors(root *cobra.Command) {
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return asUsage(err) })
	if root.Args == nil {
		root.Args = unknownCommand
	}
	wrapArgs(root)
}

func wrapArgs(cmd *cobra.Command) {
	// cobra answers a group that cannot run with its help before it reads the
	// arguments, so `wtm run bogus` exited 0. A group runs its help itself.
	if cmd.HasParent() && cmd.HasSubCommands() && !cmd.Runnable() {
		cmd.Args = unknownCommand
		cmd.RunE = func(c *cobra.Command, _ []string) error { return c.Help() }
	}
	if validate := cmd.Args; validate != nil {
		cmd.Args = func(c *cobra.Command, args []string) error { return asUsage(validate(c, args)) }
	}
	for _, sub := range cmd.Commands() {
		wrapArgs(sub)
	}
}

// validateFlagGroups runs cobra's own required-flag and flag-group checks ahead
// of cobra, which returns them raw after the hooks, past the FlagErrorFunc.
func validateFlagGroups(cmd *cobra.Command) error {
	if err := cmd.ValidateRequiredFlags(); err != nil {
		return asUsage(err)
	}
	return asUsage(cmd.ValidateFlagGroups())
}

func validateOutputFormat(cmd *cobra.Command) error {
	flag := cmd.Flags().Lookup(domain.FlagOutput)
	if flag == nil {
		return nil
	}
	formats := rules.OutputFormats(cmd.Annotations[domain.AnnotationOutputFormats])
	if slices.Contains(formats, flag.Value.String()) {
		return nil
	}
	return asUsage(fmt.Errorf(domain.OutputFormatInvalidFmt, domain.FlagOutput, flag.Value.String(), strings.Join(formats, ", ")))
}

func validateCorrelationID() error {
	if err := rules.ValidateCorrelationID(os.Getenv(domain.EnvCorrelationID)); err != nil {
		return asUsage(err)
	}
	return nil
}

func unknownCommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	message := fmt.Sprintf(domain.UnknownCommandFmt, args[0], cmd.CommandPath())
	if suggestions := cmd.SuggestionsFor(args[0]); len(suggestions) > 0 {
		message += fmt.Sprintf(domain.UnknownCommandSuggestFmt, strings.Join(suggestions, "\n\t"))
	}
	return errors.New(message)
}
