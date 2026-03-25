package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectGoSource(t *testing.T) {
	tmpDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package main
import "github.com/spf13/cobra"
`), 0o644)

	_ = os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("not go"), 0o644)

	content := collectGoSource(tmpDir)

	if !strings.Contains(content, "cobra") {
		t.Error("should contain Go source content")
	}
	if strings.Contains(content, "not go") {
		t.Error("should not contain non-Go files")
	}
}

func TestResolveDeps_UnknownLanguage(t *testing.T) {
	tmpDir := t.TempDir()

	// Should not error for unknown language
	err := ResolveDeps(tmpDir, "rust")
	if err != nil {
		t.Errorf("unexpected error for unknown language: %v", err)
	}
}
