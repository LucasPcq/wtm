package upgrade

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

type NewCmdParams struct {
	Version string
}

func NewCmd(params NewCmdParams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdUpgrade,
		Short: "Update wtm to the latest release",
		Long: "Bring wtm up to the latest published release, doing the right thing for how it was\n" +
			"installed. A standalone binary is replaced in place after its SHA256 is verified\n" +
			"against the release checksums. A Homebrew or `go install` binary is handed to that\n" +
			"tool instead — replacing a package-manager-owned binary would desynchronize it.\n" +
			"A binary built from source is refused, since no published release corresponds to it.\n" +
			"\n" +
			"This updates the CLI itself, not your worktrees — that is `wtm sync`.\n" +
			"\n" +
			"--check reports what is available without changing anything. --yes skips the\n" +
			"confirmation (required with --output json). --version pins an explicit release and\n" +
			"applies to standalone installs only.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, params.Version)
		},
	}

	cmd.Flags().Bool(domain.FlagCheck, false, "Report whether a newer release exists without installing anything")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the confirmation prompt")
	cmd.Flags().String(domain.FlagVersionPin, "", "Install a specific release instead of the latest (standalone installs only)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func run(cmd *cobra.Command, version string) error {
	check, _ := cmd.Flags().GetBool(domain.FlagCheck)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	pin, _ := cmd.Flags().GetString(domain.FlagVersionPin)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON && !yes && !check {
		return errors.New(domain.UpgradeJSONNeedsYes)
	}

	install := selfupdate.DetectInstall(version)
	if install.Method == domain.InstallSource {
		return domain.ErrUpgradeFromSource
	}
	if pin != "" && install.Method != domain.InstallStandalone {
		return errors.New(domain.UpgradePinUnsupported)
	}

	release, err := selfupdate.FetchRelease(selfupdate.FetchReleaseParams{
		Tag:       pin,
		UserAgent: userAgent(version),
		Timeout:   domain.DownloadTimeout,
	})
	// A delegated upgrade needs no release metadata to succeed: brew and go
	// resolve the version themselves, so an unreachable API must not fail a run
	// that would have worked. --check has nothing to report without it.
	if err != nil && (check || install.Method == domain.InstallStandalone) {
		return err
	}

	result := domain.UpgradeResult{
		Installed: rules.NormalizeVersion(version),
		Latest:    release.Version,
		UpToDate:  release.Version != "" && !rules.IsNewerVersion(rules.NewerVersionParams{Current: version, Latest: release.Version}),
		Method:    install.Method,
		Action:    domain.UpgradeActionNone,
	}

	if check {
		if !result.UpToDate {
			result.Action = domain.UpgradeActionChecked
		}
		return report(cmd, format, result)
	}

	if result.UpToDate && pin == "" {
		return report(cmd, format, result)
	}

	interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes
	if interactive {
		confirmed, err := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
			Title:       fmt.Sprintf(domain.UpgradeConfirmPrompt, domain.AppName, result.Installed, result.Latest),
			Description: rules.UpgradeCommandFor(install.Method),
			DefaultYes:  true,
		}))
		if err != nil {
			return err
		}
		if !confirmed {
			return domain.ErrUserAborted
		}
	}

	if err := apply(cmd, install, release, version); err != nil {
		return err
	}

	result.Action = domain.UpgradeActionReplaced
	if install.Method != domain.InstallStandalone {
		result.Action = domain.UpgradeActionDelegated
	}

	return report(cmd, format, result)
}

func apply(cmd *cobra.Command, install selfupdate.Install, release domain.ReleaseInfo, version string) error {
	if install.Method != domain.InstallStandalone {
		ran, err := selfupdate.Delegate(selfupdate.DelegateParams{
			Method: install.Method,
			Stdout: cmd.OutOrStdout(),
			Stderr: cmd.ErrOrStderr(),
		})
		if err != nil {
			return err
		}
		if !ran {
			w := cmd.ErrOrStderr()
			output.Frame(w, func(w io.Writer) {
				output.Warning(w, fmt.Sprintf("run `%s` to update", rules.UpgradeCommandFor(install.Method)))
			})
		}

		return nil
	}

	return selfupdate.ReplaceBinary(selfupdate.ReplaceBinaryParams{
		Release:    release,
		BinaryPath: install.BinaryPath,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		UserAgent:  userAgent(version),
		Timeout:    domain.DownloadTimeout,
	})
}

func report(cmd *cobra.Command, format string, result domain.UpgradeResult) error {
	w := cmd.OutOrStdout()
	if !rules.IsHumanFormat(format) {
		return output.UpgradeResultJSON(w, result)
	}

	output.Frame(w, func(w io.Writer) { output.UpgradeReport(w, result) })

	return nil
}

func userAgent(version string) string {
	return domain.AppName + "/" + rules.NormalizeVersion(version)
}
