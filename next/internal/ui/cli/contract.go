package cli

// The values and help texts of the flags every command shares, so that each command spells them alike.
const (
	OutputText  = "text" // --output's value for human output
	OutputJSON  = "json" // --output's value for the v2 envelope
	SurfaceCLI  = "cli"  // the surface an operation started from the command line records in its events
	SurfaceTUI  = "tui"  // the surface an operation started from a command's wizard records in its events
	OutputUsage = "Output format: text or json"
	DryRunUsage = "Show the plan and its fingerprint, apply nothing"
	ExpectUsage = "Apply only if the facts still have this fingerprint, the one --dry-run printed"
	AskUsage    = "Ask again the questions this repository remembers an answer to, to change or forget it"
)
