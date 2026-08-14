package rules

import "github.com/LucasPcq/wtm/internal/domain"

// InstallHooksParams holds the detection facts that decide the seeded on_create
// install hooks.
type InstallHooksParams struct {
	// RootCommand is the install command for the project root; "" omits the root
	// hook. A --install-command flag overrides detection upstream and only ever
	// sets this field — sub-projects keep their own detected command.
	RootCommand string

	// Kind classifies the repo layout. Only MonorepoKindIndependent fans out.
	Kind domain.MonorepoKind

	// SubProjects are the independently-installable directories; each contributes
	// one hook using the package manager detected in that directory.
	SubProjects []domain.SubProject
}

// BuildInstallHooks returns the on_create hooks `wtm init` seeds.
//
// A workspace root (pnpm/yarn/npm/bun workspaces, go.work, turbo/nx/lerna)
// installs every package from a single root command, so no per-package hook is
// emitted. Only a repo with no workspace declaration but per-directory lockfiles
// fans out, one hook per directory. Returns nil when nothing is installable.
func BuildInstallHooks(params InstallHooksParams) []domain.HookCommand {
	var hooks []domain.HookCommand

	if params.RootCommand != "" {
		hooks = append(hooks, domain.HookCommand{Cmd: params.RootCommand})
	}

	if params.Kind != domain.MonorepoKindIndependent {
		return hooks
	}

	for _, sub := range params.SubProjects {
		cmd := InstallCommand(sub.PackageManager)
		if cmd == "" {
			continue
		}
		hooks = append(hooks, domain.HookCommand{Cmd: cmd, Cwd: sub.Dir})
	}

	return hooks
}
