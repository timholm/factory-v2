package build

import (
	"strings"
	"testing"

	"github.com/timholm/factory-v2/internal/synthesize"
)

func TestRenderBuildPrompt(t *testing.T) {
	spec := &synthesize.ProductSpec{
		Name:     "test-tool",
		Language: "go",
	}

	prompt, err := RenderBuildPrompt(spec, "testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "github.com/testuser/test-tool") {
		t.Error("prompt should contain module path")
	}
	if !strings.Contains(prompt, "Language: go") {
		t.Error("prompt should contain language")
	}
	if !strings.Contains(prompt, "make test") {
		t.Error("prompt should mention make test")
	}
	if !strings.Contains(prompt, "SPEC.md") {
		t.Error("prompt should reference SPEC.md")
	}
}

func TestRenderBuildPrompt_DefaultLanguage(t *testing.T) {
	spec := &synthesize.ProductSpec{
		Name:     "test-tool",
		Language: "",
	}

	prompt, err := RenderBuildPrompt(spec, "testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "Language: go") {
		t.Error("empty language should default to go")
	}
}
