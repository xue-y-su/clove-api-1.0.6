package common

import "os"

func EnsurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700) // #nosec G302 -- directories need execute permission; 0700 is owner-only.
}

func OpenPrivateFile(path string, flags int) (*os.File, error) {
	file, err := os.OpenFile(path, flags, 0o600) // #nosec G304 -- callers provide operator-controlled log or profile paths.
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}
