package run

import "cmp"

// ProtocolVersion is the version of the protocol this wtm's run daemon speaks:
// a daemon stamps it on every answer, and a client reads nothing of another.
// The legacy wtm's daemon stamps none.
const ProtocolVersion = 2

// A Stamp is what a daemon writes on every answer: the Protocol it speaks
// (zero for a legacy daemon, which writes none), its build's Version, and its
// PID.
type Stamp struct {
	Protocol int
	Version  string
	PID      int
}

// Skew says how the daemon answering differs from the client asking.
type Skew string

// The skews between a client and a daemon.
const (
	SkewNone     Skew = ""         // the same build
	SkewBuild    Skew = "build"    // the same protocol, another build
	SkewProtocol Skew = "protocol" // another protocol: no answer of it is read
	SkewLegacy   Skew = "legacy"   // the legacy wtm's daemon: its listing is read as it spells it
)

// SkewOf returns how a daemon of stamp differs from a client of build version.
func SkewOf(stamp Stamp, version string) Skew {
	switch {
	case stamp.Protocol == 0:
		return SkewLegacy
	case stamp.Protocol != ProtocolVersion:
		return SkewProtocol
	case stamp.Version != version:
		return SkewBuild
	}

	return SkewNone
}

// Replaceable reports whether a daemon of skew, holding up jobs up, gives way
// to a daemon of the client's build when the client is about to start one:
// one of the same build stays, one holding no job is stopped and replaced,
// and one holding jobs stays and refuses the start, since stopping it would
// stop the foreground services someone is working in.
func Replaceable(skew Skew, up int) bool {
	return skew != SkewNone && up == 0
}

// A Skewed is what a read of the jobs says of a daemon of another build, or of
// the legacy wtm, that answered it: the jobs it lists are read all the same,
// while a start beside the jobs it holds is refused until it is restarted.
type Skewed struct {
	Skew   Skew
	Daemon string // the daemon's build, empty when it does not say
}

// Error says which daemon answered; its words name no build of the client, so they read the same from one build to the next.
func (s *Skewed) Error() string {
	daemon := "of another build (" + cmp.Or(s.Daemon, "older") + ")"
	if s.Skew == SkewLegacy {
		daemon = "the legacy wtm's"
	}
	return "the run daemon is " + daemon + ": restart it to start jobs beside its own"
}
