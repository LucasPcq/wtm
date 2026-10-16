package provision

import (
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
)

const (
	CodeEnvNoTemplate      kernel.Code = "provision.env_no_template"
	CodeEnvSourceMissing   kernel.Code = "provision.env_source_missing"
	CodeEnvParentFallback  kernel.Code = "provision.env_parent_fallback"
	CodeEnvPortIgnored     kernel.Code = "provision.env_port_ignored"
	CodeEnvValueIgnored    kernel.Code = "provision.env_value_ignored"
	CodePortsNotSettled    kernel.Code = "provision.ports_not_settled"
	CodePortsNotSettledRun kernel.Code = "provision.ports_not_settled_run"
	CodeLeftBehind         kernel.Code = "provision.left_behind"
)

const (
	ParamBranch = "branch"
	ParamCause  = "cause"
	ParamDir    = "dir"
	ParamFile   = "file"
	ParamTarget = "target"
)

var Messages = text.Messages{
	CodeEnvNoTemplate:      "no template found for {target}, skipping",
	CodeEnvSourceMissing:   "{target} not found in {dir}, skipping",
	CodeEnvParentFallback:  "{target} not found in parent worktree, falling back to main",
	CodeEnvPortIgnored:     "env_port {key} in {file} ignored: not a configured env file — add it to [env] in config.toml or drop the link",
	CodeEnvValueIgnored:    "env {key} in {file} ignored: not a configured env file — add it to [env] in config.toml or drop the link",
	CodePortsNotSettled:    "{cause} — ports not settled — run `wtm env {branch}` once this is fixed",
	CodePortsNotSettledRun: "{cause} — ports not settled — run `wtm env {branch}` once run.toml is fixed",
	CodeLeftBehind:         "{branch} is left half-created at {path}: remove it with `wtm clean --force {branch}`",
}
