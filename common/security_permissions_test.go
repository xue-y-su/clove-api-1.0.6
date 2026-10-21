package common

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsurePrivateDirCreatesOwnerOnlyDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX directory permission bits")
	}

	dir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, EnsurePrivateDir(dir))

	info, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestOpenPrivateFileCreatesOwnerOnlyFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX file permission bits")
	}

	path := filepath.Join(t.TempDir(), "private.log")
	file, err := OpenPrivateFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
