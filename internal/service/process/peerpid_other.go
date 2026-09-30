//go:build !darwin && !linux

package process

import "errors"

func peerPIDOf(uintptr) (int, error) {
	return 0, errors.New("peer pid: unsupported platform")
}
