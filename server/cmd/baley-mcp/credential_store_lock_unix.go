//go:build !windows

package main

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func acquireCredentialStoreFileLock(_ context.Context, path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open Baley credential lock: %w", err)
	}
	if err = unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("lock Baley credential store: %w", err)
	}
	return func() {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
	}, nil
}
