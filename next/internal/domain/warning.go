package domain

import "github.com/LucasPcq/wtm/next/internal/domain/env"

// Warning is something the user should know about an operation that still succeeded: what to do about it, if anything, as a follow-up, and the env ports it is about, if any.
type Warning struct {
	Code     string      `json:"code"`
	Params   Params      `json:"params,omitempty"`
	Message  string      `json:"message"`
	FollowUp *FollowUp   `json:"follow_up,omitempty"`
	Ports    []env.Entry `json:"ports,omitempty"`
}

// LocksUnavailable warns that the operation ran, or a file was written, without holding its locks.
var LocksUnavailable = Warning{Code: "locks.unavailable", Message: "ran without operation locks"}
