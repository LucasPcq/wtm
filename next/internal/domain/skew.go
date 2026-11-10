package domain

import (
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// SkewWarningParams are a daemon of another build that answered the command named Command, and the follow-up that restarts it.
type SkewWarningParams struct {
	Command string
	Skewed  *run.Skewed
	Restart *FollowUp
}

// SkewWarning is <command>.daemon_skew: the daemon that answered is of another build, or the legacy wtm's, its skew and build in its params.
func SkewWarning(params SkewWarningParams) Warning {
	warning := Warning{Code: params.Command + DaemonSkew, Message: params.Skewed.Error(), Params: Params{"skew": string(params.Skewed.Skew), "daemon": params.Skewed.Daemon}}
	if params.Restart != nil {
		restart := *params.Restart
		restart.Code, restart.Message = params.Command+".restart_daemon", "restart it: its jobs come back on a daemon of this build"
		warning.FollowUp = &restart
	}
	return warning
}

// SkewRefusalParams are the command refused, what it did not do (its message's start), the error wtm's daemon's client failed on, and the command that hands a skewed daemon's jobs over.
type SkewRefusalParams struct {
	Command string
	Undone  string
	Err     error
	Restart *FollowUp
}

// SkewRefusal is <command>.daemon_skew as a refusal (KindPrecondition), followed by the restart, when Err is a daemon of another build that answered (a *run.Skewed) or wtm's daemon's client failing on a daemon of another protocol or build, or the legacy wtm's, that it cannot go on with (run.CodeSkew, run.CodeLegacy); nil for any other error.
func SkewRefusal(params SkewRefusalParams) *Error {
	skewed, ok := errors.AsType[*run.Skewed](params.Err)
	if !ok {
		refused, ok := errors.AsType[*run.Error](params.Err)
		if !ok || refused.Code != run.CodeSkew && refused.Code != run.CodeLegacy {
			return nil
		}
		skewed = &run.Skewed{Skew: run.SkewBuild}
		if refused.Code == run.CodeLegacy {
			skewed.Skew = run.SkewLegacy
		}
	}

	warning := SkewWarning(SkewWarningParams{Command: params.Command, Skewed: skewed, Restart: params.Restart})
	return &Error{Kind: KindPrecondition, Code: warning.Code, Message: params.Undone + ": " + errorText(params.Err), Params: warning.Params, FollowUp: warning.FollowUp, Cause: params.Err}
}

// errorText is what err says, the daemon's own words for a refusal of its client.
func errorText(err error) string {
	if refused, ok := errors.AsType[*run.Error](err); ok {
		return refused.Message
	}
	return err.Error()
}

// IndexFrozenWarning is <command>.index_frozen: the daemon's index of jobs, at path, was written by a newer wtm, so the daemon of this one records nothing it starts: a stack it starts is not taken back by the next daemon, nor a job it runs brought back by a restart.
func IndexFrozenWarning(command, path string) Warning {
	return Warning{Code: command + IndexFrozen, Params: Params{"path": path}, Message: "the index of jobs " + path + " was written by a newer wtm: nothing started here is recorded, so no restart brings it back — run the newer wtm, or remove the file once nothing runs"}
}
