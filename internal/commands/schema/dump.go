package schema

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/schemas"
)

func newDumpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "dump",
		Annotations: map[string]string{domain.AnnotationMachineOutput: domain.AnnotationOn},
		Short:       "Write embedded schemas to <state-dir>/schemas/ (or the global config's schemas/ with --global)",
		Long:        "Extract every JSON Schema bundled with this wtm binary so editors can resolve the `#:schema` directives in your TOML files.\nProject schemas land in <git-common-dir>/wtm/schemas/. Use --global to write the global schema next to the global wtm config, whose path `wtm run proxy status` prints.",
		RunE:        runDump,
	}
	cmd.Flags().Bool(domain.FlagGlobal, false, "Write the global config schema instead of the project ones")
	return cmd
}

func runDump(cmd *cobra.Command, _ []string) error {
	global, _ := cmd.Flags().GetBool(domain.FlagGlobal)

	if global {
		dir, err := infra.GlobalDir()
		if err != nil {
			return err
		}
		schemaDir := filepath.Join(dir, domain.SchemasDirName)
		written, err := writeSchemas(schemaDir, []schemas.Schema{schemas.Global})
		if err != nil {
			return err
		}
		output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			for _, p := range written {
				output.Success(w, fmt.Sprintf("Wrote %s", p))
			}
		})
		return nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	stateDir, err := shared.StateDir(wd)
	if err != nil {
		return err
	}
	schemaDir := filepath.Join(stateDir, domain.SchemasDirName)
	written, err := writeSchemas(schemaDir, []schemas.Schema{schemas.Project, schemas.Run})
	if err != nil {
		return err
	}
	output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		for _, p := range written {
			output.Success(w, fmt.Sprintf("Wrote %s", p))
		}
	})
	return nil
}

// writeSchemas extracts the given schemas into dir, creating it if needed.
func writeSchemas(dir string, list []schemas.Schema) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	written := make([]string, 0, len(list))
	for _, s := range list {
		path := filepath.Join(dir, s.Filename())
		if err := os.WriteFile(path, s.Bytes(), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", path, err)
		}
		written = append(written, path)
	}
	return written, nil
}
