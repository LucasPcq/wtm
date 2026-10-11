package dispatch

import "github.com/LucasPcq/wtm/internal/kernel"

const (
	CodeLockHeld         kernel.Code = "lock.held"
	CodePlanStale        kernel.Code = "plan.stale"
	CodeStateSchemaNewer kernel.Code = "state.schema_newer"
	CodeLocksUnavailable kernel.Code = "locks.unavailable"
	CodeUnknownField     kernel.Code = "request.unknown_field"
)

const (
	ParamKey       = "key"
	ParamOp        = "op"
	ParamCommand   = "command"
	ParamPID       = "pid"
	ParamSurface   = "surface"
	ParamStartedAt = "started_at"
	ParamExpected  = "expected"
	ParamFound     = "found"
	ParamSupported = "supported"
)
