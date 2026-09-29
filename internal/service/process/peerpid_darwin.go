package process

import "syscall"

// solLocal and localPeerPID are <sys/un.h>'s SOL_LOCAL and LOCAL_PEERPID, which
// the syscall package does not name.
const (
	solLocal     = 0
	localPeerPID = 0x002
)

func peerPIDOf(fd uintptr) (int, error) {
	return syscall.GetsockoptInt(int(fd), solLocal, localPeerPID)
}
