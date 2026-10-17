package cli

const (
	OutputText  = "text"
	OutputJSON  = "json"
	SurfaceCLI  = "cli"
	OutputUsage = "Output format: text or json"
	DryRunUsage = "Show the plan and its fingerprint, apply nothing"
	ExpectUsage = "Apply only if the facts still have this fingerprint, the one --dry-run printed"
	AskUsage    = "Ask again the questions this repository remembers an answer to, to change or forget it"
)
