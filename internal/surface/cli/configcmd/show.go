package configcmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Print the project config.toml",
		Example: `  wtm config show

  # Check the file, print nothing else
  wtm config show --validate

  wtm config show --output json`,
		RunE: runShow,
	}
	shared.AddOutputFlag(cmd)
	cmd.Flags().Bool(domain.FlagValidate, false, "Validate the config instead of printing it")
	return cmd
}

func runShow(cmd *cobra.Command, _ []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	stateDir, err := shared.StateDir(cmd.Context(), wd)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	validate, _ := cmd.Flags().GetBool(domain.FlagValidate)

	if validate {
		return runValidate(cmd, stateDir, format)
	}

	if format == domain.OutputJSON {
		return runShowJSON(cmd, stateDir)
	}

	return runShowText(cmd, stateDir)
}

// runShowJSON emits the structured project config as JSON. A missing config is
// reported as an empty object so agents can branch without parsing an error.
func runShowJSON(cmd *cobra.Command, stateDir string) error {
	cfg, err := config.LoadProjectRaw(stateDir)
	if errors.Is(err, domain.ErrConfigNotFound) {
		return render.WriteProjectConfigJSON(cmd.OutOrStdout(), domain.ProjectConfig{})
	}
	if err != nil {
		return err
	}
	return render.WriteProjectConfigJSON(cmd.OutOrStdout(), cfg)
}

// runValidate loads and validates the merged config, reporting the outcome in the
// requested format.
func runValidate(cmd *cobra.Command, stateDir string, format string) error {
	_, err := config.Load(config.LoadParams{StateDir: stateDir})

	if format == domain.OutputJSON {
		payload := render.ConfigValidateJSON{Valid: err == nil}
		if err != nil {
			payload.Error = err.Error()
		}
		return render.WriteConfigValidateJSON(cmd.OutOrStdout(), payload)
	}

	if err != nil {
		render.Frame(cmd.ErrOrStderr(), func(w io.Writer) {
			render.Error(w, err.Error())
		})
		return domain.ErrAborted
	}
	render.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		render.Success(w, "Config is valid.")
	})
	return nil
}

// runShowText prints the config path followed by the raw TOML body.
func runShowText(cmd *cobra.Command, stateDir string) error {
	path := filepath.Join(stateDir, domain.ConfigFileName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		render.Frame(cmd.ErrOrStderr(), func(w io.Writer) {
			render.Warning(w, fmt.Sprintf("No config at %s. Run `wtm init` first.", path))
		})
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var writeErr error
	render.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		render.InfoLine(w, "path", path)
		render.Blank(w)
		_, writeErr = w.Write(data)
	})
	if writeErr != nil {
		return writeErr
	}
	return nil
}
