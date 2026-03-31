package qqbot

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ProcessLock manages a PID file to ensure only one qqbot channel instance
// runs per config. A new instance can take over from an existing one by
// sending SIGTERM and waiting for it to exit.
type ProcessLock struct {
	path string
}

// NewProcessLock creates a lock associated with the given PID file path.
func NewProcessLock(path string) *ProcessLock {
	return &ProcessLock{path: path}
}

// Acquire obtains the lock. If another live process holds the lock, it is
// terminated with SIGTERM first (takeover). If the holder is already dead,
// the stale lock is cleaned up.
func (l *ProcessLock) Acquire() error {
	for {
		holderPID, err := l.readPID()
		if err != nil {
			// No lock file or corrupt — safe to create.
			return l.writePID()
		}

		if holderAlive(holderPID) {
			// Terminate the previous holder and wait for it to release the lock.
			if err := l.takeover(holderPID); err != nil {
				return fmt.Errorf("takeover from pid %d: %w", holderPID, err)
			}
			// Loop back — the lock file should now be gone or stale.
			continue
		}

		// Holder is dead — stale lock, remove and create our own.
		os.Remove(l.path)
		return l.writePID()
	}
}

// Release removes the PID file.
func (l *ProcessLock) Release() {
	os.Remove(l.path)
}

// readPID returns the PID stored in the lock file, or an error if absent.
func (l *ProcessLock) readPID() (int, error) {
	data, err := os.ReadFile(l.path)
	if os.IsNotExist(err) {
		return 0, fmt.Errorf("no lock file")
	}
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid pid in lock file: %q", s)
	}
	return pid, nil
}

// writePID writes the current process PID to the lock file.
func (l *ProcessLock) writePID() error {
	return os.WriteFile(l.path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
}

// takeover sends SIGTERM to the holder and waits for it to exit or for the
// lock file to be removed.
func (l *ProcessLock) takeover(holderPID int) error {
	proc, err := os.FindProcess(holderPID)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// Process may have already exited between the check and signal.
		return nil
	}

	// Wait for the lock file to disappear (holder releases on shutdown).
	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("holder %d did not exit within 10s", holderPID)
		case <-ticker.C:
			if _, err := os.Stat(l.path); os.IsNotExist(err) {
				return nil
			}
			// Also check if holder died without cleaning up.
			if !holderAlive(holderPID) {
				os.Remove(l.path)
				return nil
			}
		}
	}
}

// holderAlive reports whether the process with the given PID is still running.
func holderAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; signal 0 checks liveness.
	return proc.Signal(syscall.Signal(0)) == nil
}
