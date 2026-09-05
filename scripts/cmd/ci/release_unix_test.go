//go:build linux || darwin

package main

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestHashPluginTreeRejectsSpecialFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "named-pipe")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := hashPluginTree(root); err == nil || !strings.Contains(err.Error(), "special file") {
		t.Fatalf("expected special-file rejection, got %v", err)
	}
}
