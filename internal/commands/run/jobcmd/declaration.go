package jobcmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// declarationHelp is the wording of the flags add and edit share; edit's says
// how each one is withdrawn, which add has no use for.
type declarationHelp struct {
	Runs, Touches, BindsNoPort, Scope string
	NamespaceName, NamespaceCreate    string
	NamespaceRemove, NamespaceEnv     string
}

var addDeclarationHelp = declarationHelp{
	Runs:            "Declared job this one starts itself, repeatable (a turbo or compose runner)",
	Touches:         "Declared service whose data this job changes (a migration, a reset, a seed), repeatable",
	BindsNoPort:     "This service listens on nothing by design, so stop offering it a port",
	Scope:           "shared runs one instance for the whole repository; worktree (the default) one per worktree",
	NamespaceName:   "Name of each worktree's slice of a shared service, e.g. app_{worktree}",
	NamespaceCreate: "Command carving the slice out, run on every start of the shared service (must be safe to rerun)",
	NamespaceRemove: "Command dropping the slice, run by wtm clean",
	NamespaceEnv:    "Extra variable for the namespace commands as KEY=VALUE, repeatable ({worktree} and {ordinal} are filled in)",
}

var editDeclarationHelp = declarationHelp{
	Runs:            "Declared job this one starts itself, repeatable — replaces the list (pass '' to drop it)",
	Touches:         "Declared service whose data this job changes (a migration, a reset, a seed), repeatable — replaces the list (pass '' to drop it)",
	BindsNoPort:     "This service listens on nothing by design, so stop offering it a port (--binds-no-port=false to undo)",
	Scope:           "shared runs one instance for the whole repository; worktree one per worktree",
	NamespaceName:   "Name of each worktree's slice of a shared service (pass '' to withdraw the whole [job.namespace])",
	NamespaceCreate: "Command carving the slice out, run on every start of the shared service (must be safe to rerun)",
	NamespaceRemove: "Command dropping the slice, run by wtm clean (pass '' to drop it)",
	NamespaceEnv:    "Extra variable for the namespace commands as KEY=VALUE, repeatable — replaces the table (pass '' to drop it)",
}

func addDeclarationFlags(cmd *cobra.Command, help declarationHelp) {
	cmd.Flags().StringArray(domain.FlagRuns, nil, help.Runs)
	cmd.Flags().StringArray(domain.FlagTouches, nil, help.Touches)
	cmd.Flags().Bool(domain.FlagBindsNoPort, false, help.BindsNoPort)
	cmd.Flags().String(domain.FlagScope, "", help.Scope)
	cmd.Flags().String(domain.FlagNamespaceName, "", help.NamespaceName)
	cmd.Flags().String(domain.FlagNamespaceCreate, "", help.NamespaceCreate)
	cmd.Flags().String(domain.FlagNamespaceRemove, "", help.NamespaceRemove)
	cmd.Flags().StringArray(domain.FlagNamespaceEnv, nil, help.NamespaceEnv)
}

// declarationPatch reads only the flags actually passed, so an absent flag can
// be told from an explicit empty value.
func declarationPatch(cmd *cobra.Command) rules.JobPatch {
	var patch rules.JobPatch
	for _, field := range []struct {
		flag string
		into **string
	}{
		{domain.FlagScope, &patch.Scope},
		{domain.FlagNamespaceName, &patch.NamespaceName},
		{domain.FlagNamespaceCreate, &patch.NamespaceCreate},
		{domain.FlagNamespaceRemove, &patch.NamespaceRemove},
	} {
		if !cmd.Flags().Changed(field.flag) {
			continue
		}
		value, _ := cmd.Flags().GetString(field.flag)
		*field.into = &value
	}
	for _, field := range []struct {
		flag string
		into **[]string
	}{
		{domain.FlagRuns, &patch.Runs},
		{domain.FlagTouches, &patch.Touches},
		{domain.FlagNamespaceEnv, &patch.NamespaceEnv},
	} {
		if !cmd.Flags().Changed(field.flag) {
			continue
		}
		values, _ := cmd.Flags().GetStringArray(field.flag)
		*field.into = &values
	}
	if cmd.Flags().Changed(domain.FlagBindsNoPort) {
		binds, _ := cmd.Flags().GetBool(domain.FlagBindsNoPort)
		patch.BindsNoPort = &binds
	}
	return patch
}
