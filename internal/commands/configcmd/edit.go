package configcmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
)

const defaultEditor = "vi"

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open the project config.toml in $EDITOR",
		Long:  "Launch the editor on <git-common-dir>/wtm/config.toml. After save, the file is re-validated and any error is reported.",
		RunE:  runEdit,
	}
}

func runEdit(cmd *cobra.Command, _ []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	stateDir, err := shared.StateDir(wd)
	if err != nil {
		return err
	}

	path := filepath.Join(stateDir, domain.ConfigFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		output.Frame(cmd.ErrOrStderr(), func(w io.Writer) {
			output.Warning(w, fmt.Sprintf("No config at %s. Run `wtm init` first.", path))
		})
		return nil
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = defaultEditor
	}

	editCmd := exec.Command(editor, path)
	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr
	if err := editCmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	if _, err := config.Load(config.LoadParams{StateDir: stateDir}); err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			output.Frame(cmd.ErrOrStderr(), func(w io.Writer) {
				output.Warning(w, "Config file is missing after edit.")
			})
			return nil
		}
		output.Frame(cmd.ErrOrStderr(), func(w io.Writer) {
			output.Error(w, fmt.Sprintf("Config no longer valid: %v", err))
		})
		// The block above IS the report — returning err would have Execute spell
		// the same message a second time, unframed.
		return fmt.Errorf("%w: %w", domain.ErrAborted, err)
	}

	output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		output.Success(w, "Config saved and validated.")
	})
	return nil
}
