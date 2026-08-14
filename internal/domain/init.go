package domain

// PackageManager represents a detected package manager.
type PackageManager string

const (
	PkgManagerPnpm PackageManager = "pnpm"
	PkgManagerNpm  PackageManager = "npm"
	PkgManagerYarn PackageManager = "yarn"
	PkgManagerGo   PackageManager = "go"
	PkgManagerPip  PackageManager = "pip"
	PkgManagerNone PackageManager = "none"
)

// MonorepoKind classifies how a repository installs its dependencies.
type MonorepoKind string

const (
	// MonorepoKindNone is a plain single project: one install at the root.
	MonorepoKindNone MonorepoKind = "none"

	// MonorepoKindWorkspace has a root workspace declaration (pnpm-workspace.yaml,
	// package.json "workspaces", go.work, turbo/nx/lerna). The root install already
	// covers every package, so no per-package hook is seeded.
	MonorepoKindWorkspace MonorepoKind = "workspace"

	// MonorepoKindIndependent has no workspace declaration but sub-directories that
	// carry their own lockfile; each installs on its own.
	MonorepoKindIndependent MonorepoKind = "independent"
)

// SubProject is an independently-installable directory below the project root.
// Dir is slash-separated and relative to the root.
type SubProject struct {
	Dir            string
	PackageManager PackageManager
}

// MonorepoLayout holds the detected facts that decide the install-hook list.
// WorkspacePackages is informational (workspace kind only); SubProjects is the
// only field that drives hook fan-out.
type MonorepoLayout struct {
	Kind              MonorepoKind
	WorkspacePackages []string
	SubProjects       []SubProject
}

// PackageScript is one script entry discovered via package.json, possibly
// inside a workspace package.
type PackageScript struct {
	Name      string  // script name, e.g. "dev"
	Cmd       string  // raw script value from package.json
	Workspace string  // relative dir of the package, "" for root
	PkgName   string  // from package.json "name" field, npm scope stripped
	Kind      JobKind // pre-classified kind; used to drive default selection in the wizard
}

// InitDetectionResult holds all auto-detected values for repo init.
type InitDetectionResult struct {
	BaseBranch         string
	Branches           []BranchCandidate
	EnvFiles           []EnvFile
	PackageManager     PackageManager
	InstallCommand     string
	DockerComposeFiles []string
	DockerComposeCmd   string
	Monorepo           MonorepoLayout
	PackageScripts     []PackageScript
}

// InitGlobalAnswers holds the wizard answers for global config setup.
type InitGlobalAnswers struct {
	Shell ShellType
}

// RecapField is one aligned "label   value" row of a framed command recap (the
// init config summary, the create result fields, etc.).
type RecapField struct {
	Label string
	Value string
}

// InitProjectAnswers holds the wizard answers for project config setup.
// The Skip* flags record sections the user opted out of (via the wizard skip
// key or the --skip-* flags); they drive whether each section is written as
// active config or left commented as a template.
type InitProjectAnswers struct {
	BasePath               string
	BaseBranch             string
	EnvFiles               []EnvFile
	EnvStrategy            EnvStrategy
	OnCreate               []HookCommand
	OnClean                []HookCommand
	DockerComposeFiles     []string
	DockerComposeCmd       string
	SelectedPackageScripts []PackageScript
	SkipEnv                bool
	SkipHooks              bool
	SkipClean              bool
}
