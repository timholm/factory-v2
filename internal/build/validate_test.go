package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/timholm/factory-v2/internal/synthesize"
)

func TestValidate_MissingGoMod(t *testing.T) {
	tmpDir := t.TempDir()

	spec := &synthesize.ProductSpec{
		Name:     "test-tool",
		Language: "go",
		Papers: []synthesize.PaperRef{
			{ID: "2401.00001", Title: "Paper 1"},
		},
	}

	err := Validate(tmpDir, spec, "testuser")
	if err == nil {
		t.Fatal("expected validation error for missing go.mod")
	}
}

func TestValidate_MissingReadme(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module github.com/testuser/test-tool\n"), 0o644)

	spec := &synthesize.ProductSpec{
		Name:     "test-tool",
		Language: "go",
		Papers: []synthesize.PaperRef{
			{ID: "2401.00001", Title: "Paper 1"},
		},
	}

	err := Validate(tmpDir, spec, "testuser")
	if err == nil {
		t.Fatal("expected validation error for missing README")
	}
}

func TestValidate_Pass(t *testing.T) {
	tmpDir := t.TempDir()

	spec := &synthesize.ProductSpec{
		Name:     "test-tool",
		Language: "go",
		Papers: []synthesize.PaperRef{
			{ID: "2401.00001", Title: "Paper 1"},
			{ID: "2401.00002", Title: "Paper 2"},
			{ID: "2401.00003", Title: "Paper 3"},
		},
	}

	// Create go.mod
	_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"),
		[]byte("module github.com/testuser/test-tool\n"), 0o644)

	// Create README with references
	readme := "# test-tool\n\nReferences:\n- 2401.00001\n- 2401.00002\n- 2401.00003\n"
	_ = os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(readme), 0o644)

	// Create test file
	_ = os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0o755)
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg", "core_test.go"), []byte("package pkg\n"), 0o644)

	err := Validate(tmpDir, spec, "testuser")
	if err != nil {
		t.Fatalf("validation should pass: %v", err)
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	if fileExists(filepath.Join(tmpDir, "nonexistent")) {
		t.Error("nonexistent file should not exist")
	}

	f := filepath.Join(tmpDir, "exists.txt")
	_ = os.WriteFile(f, []byte("hi"), 0o644)
	if !fileExists(f) {
		t.Error("created file should exist")
	}
}

func TestCheckNoSecrets(t *testing.T) {
	tmpDir := t.TempDir()

	// Clean directory
	_ = os.WriteFile(filepath.Join(tmpDir, "clean.go"), []byte("package main\n"), 0o644)
	if err := checkNoSecrets(tmpDir); err != nil {
		t.Errorf("clean dir should pass: %v", err)
	}
}
