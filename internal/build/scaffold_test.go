package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timholm/factory-v2/internal/synthesize"
)

func TestScaffold(t *testing.T) {
	tmpDir := t.TempDir()

	spec := &synthesize.ProductSpec{
		Name:        "test-tool",
		Description: "A test tool for testing",
		Language:    "go",
		Papers: []synthesize.PaperRef{
			{ID: "2401.00001", Title: "Paper 1"},
		},
		Repos: []synthesize.RepoRef{
			{FullName: "owner/repo", URL: "https://github.com/owner/repo"},
		},
		Techniques:   []string{"transformer"},
		TechniqueMap: map[string]string{"transformer": "core engine"},
		Architecture: "Pipeline architecture with three stages",
		Features:     []string{"feature 1", "feature 2"},
	}

	err := Scaffold(tmpDir, spec, "testuser")
	if err != nil {
		t.Fatalf("scaffold failed: %v", err)
	}

	// Check SPEC.md exists and has content
	specMD, err := os.ReadFile(filepath.Join(tmpDir, "SPEC.md"))
	if err != nil {
		t.Fatalf("SPEC.md not found: %v", err)
	}
	if !strings.Contains(string(specMD), "test-tool") {
		t.Error("SPEC.md should contain tool name")
	}
	if !strings.Contains(string(specMD), "2401.00001") {
		t.Error("SPEC.md should contain paper ID")
	}

	// Check go.mod
	gomod, err := os.ReadFile(filepath.Join(tmpDir, "go.mod"))
	if err != nil {
		t.Fatalf("go.mod not found: %v", err)
	}
	if !strings.Contains(string(gomod), "github.com/testuser/test-tool") {
		t.Error("go.mod should have correct module path")
	}

	// Check Makefile
	makefile, err := os.ReadFile(filepath.Join(tmpDir, "Makefile"))
	if err != nil {
		t.Fatalf("Makefile not found: %v", err)
	}
	if !strings.Contains(string(makefile), "go build") {
		t.Error("Makefile should contain go build")
	}

	// Check CLAUDE.md
	if _, err := os.Stat(filepath.Join(tmpDir, "CLAUDE.md")); err != nil {
		t.Error("CLAUDE.md not found")
	}

	// Check .gitignore
	if _, err := os.Stat(filepath.Join(tmpDir, ".gitignore")); err != nil {
		t.Error(".gitignore not found")
	}

	// Check spec.json
	if _, err := os.Stat(filepath.Join(tmpDir, "spec.json")); err != nil {
		t.Error("spec.json not found")
	}
}

func TestRenderMakefile(t *testing.T) {
	goMake := renderMakefile("go")
	if !strings.Contains(goMake, "go build") {
		t.Error("go makefile should contain go build")
	}

	pyMake := renderMakefile("python")
	if !strings.Contains(pyMake, "pytest") {
		t.Error("python makefile should contain pytest")
	}

	tsMake := renderMakefile("typescript")
	if !strings.Contains(tsMake, "tsc") {
		t.Error("typescript makefile should contain tsc")
	}
}

func TestWriteFile(t *testing.T) {
	tmpDir := t.TempDir()

	err := writeFile(tmpDir, "test.txt", "hello world")
	if err != nil {
		t.Fatalf("writeFile failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("unexpected content: %s", string(data))
	}
}

func TestWriteFileNestedDir(t *testing.T) {
	tmpDir := t.TempDir()

	err := writeFile(tmpDir, "sub/dir/test.txt", "nested")
	if err != nil {
		t.Fatalf("writeFile failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "sub", "dir", "test.txt"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) != "nested" {
		t.Errorf("unexpected content: %s", string(data))
	}
}
