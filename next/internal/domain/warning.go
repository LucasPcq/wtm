package domain

// Warning is something the user should know about an operation that still succeeded.
type Warning struct {
	Code    string `json:"code"`
	Params  Params `json:"params,omitempty"`
	Message string `json:"message"`
}

// LocksUnavailable warns that the operation ran, or a file was written, without holding its locks.
var LocksUnavailable = Warning{Code: "locks.unavailable", Message: "ran without operation locks"}
