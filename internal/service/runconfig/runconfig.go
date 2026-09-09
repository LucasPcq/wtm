// Package runconfig orchestrates load + validate + write on the run.toml
// config file. Mirrors the layering of internal/service/worktree: pure I/O
// over config/ + rules/, no cobra/bubbletea.
package runconfig

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/shellcmd"
)

// Load reads run.toml from the state directory. Returns an empty RunConfig
// (no error) when the file is absent — callers can mutate-and-Save to
// initialise the file.
func Load(stateDir string) (domain.RunConfig, error) {
	return config.LoadRun(stateDir)
}

// SaveParams holds the inputs for Save.
type SaveParams struct {
	StateDir string
	Config   domain.RunConfig
}

// Save validates cfg via rules.ValidateRun and writes it to <stateDir>/run.toml,
// overwriting any previous content. Validation errors are joined with "; "
// and prefixed "invalid run config: ".
func Save(params SaveParams) error {
	_, errs := rules.ValidateRun(params.Config)
	errs = append(errs, shellSyntaxErrors(params.Config)...)
	errs = append(errs, rules.ValidateTenants(params.Config)...)
	errs = append(errs, rules.ValidateJobNames(params.Config)...)
	if len(errs) > 0 {
		return fmt.Errorf("invalid run config: %s", strings.Join(errs, "; "))
	}
	return config.WriteRun(config.WriteRunParams{
		StateDir: params.StateDir,
		Config:   params.Config,
		Force:    true,
	})
}

// shellSyntaxErrors rejects the commands the shell could not parse. Every write
// path goes through Save, so this is the one moment a broken quote can still be
// named — after it, the failure surfaces as a job that dies at startup.
func shellSyntaxErrors(cfg domain.RunConfig) []string {
	var errs []string
	for _, job := range cfg.Jobs {
		for _, field := range []struct {
			name string
			line string
		}{{domain.FlagCmd, job.Cmd}, {domain.FlagStop, job.Stop}} {
			if rules.IsBlankCommand(field.line) {
				continue
			}
			if err := shellcmd.CheckSyntax(field.line); err != nil {
				errs = append(errs, fmt.Sprintf("job %q: %s is not a valid shell command: %v", job.Name, field.name, err))
			}
		}
	}
	return errs
}
