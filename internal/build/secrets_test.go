package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScrubSecrets(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with secrets
	content := `package main

const token = "ghp_ABC123DEF456GHI789JKL012MNO345PQR678"
const apiKey = "sk-ant-api03-abcdefghijklmnop"
const openaiKey = "sk-12345678901234567890"
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	ScrubSecrets(tmpDir)

	data, err := os.ReadFile(filepath.Join(tmpDir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}

	result := string(data)

	if strings.Contains(result, "ghp_ABC") {
		t.Error("GitHub token should be scrubbed")
	}
	if strings.Contains(result, "sk-ant-api03") {
		t.Error("Anthropic key should be scrubbed")
	}
	if strings.Contains(result, "sk-1234567890") {
		t.Error("OpenAI key should be scrubbed")
	}
	if !strings.Contains(result, "REDACTED") {
		t.Error("secrets should be replaced with REDACTED")
	}
}

func TestScrubSecretsSkipsBinary(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a binary-like file
	err := os.WriteFile(filepath.Join(tmpDir, "app.exe"), []byte("ghp_ABC123DEF456GHI789JKL012MNO345PQR678"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	ScrubSecrets(tmpDir)

	// Binary file should NOT be modified
	data, err := os.ReadFile(filepath.Join(tmpDir, "app.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "REDACTED") {
		t.Error("binary files should not be scrubbed")
	}
}

func TestScrubSecretsCleanFile(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package main

func main() {
    fmt.Println("no secrets here")
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "clean.go"), []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	ScrubSecrets(tmpDir)

	data, err := os.ReadFile(filepath.Join(tmpDir, "clean.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Error("clean file should not be modified")
	}
}
