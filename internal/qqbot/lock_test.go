package qqbot

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAcquireNoExisting(t *testing.T) {
	dir := t.TempDir()
	lock := NewProcessLock(filepath.Join(dir, "test.pid"))

	if err := lock.Acquire(); err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	defer lock.Release()

	data, err := os.ReadFile(lock.path)
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := strconv.Atoi(string(data[:len(data)-1])) // strip newline
	if pid != os.Getpid() {
		t.Errorf("PID = %d, want %d", pid, os.Getpid())
	}
}

func TestRelease(t *testing.T) {
	dir := t.TempDir()
	lock := NewProcessLock(filepath.Join(dir, "test.pid"))

	if err := lock.Acquire(); err != nil {
		t.Fatal(err)
	}
	lock.Release()

	if _, err := os.Stat(lock.path); !os.IsNotExist(err) {
		t.Errorf("lock file still exists after Release()")
	}
}

func TestAcquireStaleLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	// Write a PID that does not exist.
	os.WriteFile(path, []byte("99999999\n"), 0644)

	lock := NewProcessLock(path)
	if err := lock.Acquire(); err != nil {
		t.Fatalf("Acquire() with stale lock: %v", err)
	}
	defer lock.Release()

	data, _ := os.ReadFile(path)
	pid, _ := strconv.Atoi(string(data[:len(data)-1]))
	if pid != os.Getpid() {
		t.Errorf("PID = %d after stale cleanup, want %d", pid, os.Getpid())
	}
}

func TestAcquireCorruptLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	os.WriteFile(path, []byte("not-a-pid\n"), 0644)

	lock := NewProcessLock(path)
	if err := lock.Acquire(); err != nil {
		t.Fatalf("Acquire() with corrupt lock: %v", err)
	}
	defer lock.Release()
}

func TestHolderAlive(t *testing.T) {
	if !holderAlive(os.Getpid()) {
		t.Error("holderAlive(self) = false, want true")
	}
	if holderAlive(99999999) {
		t.Error("holderAlive(99999999) = true, want false")
	}
}

func TestReadPID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	lock := NewProcessLock(path)

	// Non-existent file.
	pid, err := lock.readPID()
	if err == nil {
		t.Error("expected error for non-existent file")
	}

	// Valid file.
	os.WriteFile(path, []byte("12345\n"), 0644)
	pid, err = lock.readPID()
	if err != nil {
		t.Fatalf("readPID() error: %v", err)
	}
	if pid != 12345 {
		t.Errorf("readPID() = %d, want 12345", pid)
	}
}
