package domain

type Warning struct {
	Code    string `json:"code"`
	Params  Params `json:"params,omitempty"`
	Message string `json:"message"`
}

var LocksUnavailable = Warning{Code: "locks.unavailable", Message: "ran without operation locks"}
