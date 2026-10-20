package output

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestTheReaderOfAPipeLeavingIsNoticed(t *testing.T) {
	read, write, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = write.Close() })
	gone := fileStream(write).Gone(t.Context())

	require.NoError(t, read.Close())

	assert.True(t, closesSoon(gone))
}

// Legacy watched pipes alone: a stream to a socket, as a Node child's stdio is, outlived its reader.
func TestTheReaderOfASocketLeavingIsNoticed(t *testing.T) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	require.NoError(t, err)
	ours, theirs := os.NewFile(uintptr(fds[0]), "ours"), os.NewFile(uintptr(fds[1]), "theirs")
	t.Cleanup(func() { _ = ours.Close() })
	gone := fileStream(ours).Gone(t.Context())

	require.NoError(t, theirs.Close())

	assert.True(t, closesSoon(gone))
}

func TestAReaderStillThereIsNotGone(t *testing.T) {
	read, write, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = read.Close(); _ = write.Close() })

	assert.False(t, closesSoon(fileStream(write).Gone(t.Context())))
}

func TestAStreamOverNoFileNeverSaysItsReaderLeft(t *testing.T) {
	streams, _, _, _ := Test()

	assert.False(t, closesSoon(streams.Out.Gone(t.Context())))
}
