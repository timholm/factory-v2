package build

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// secretPatterns are regex patterns for secrets that must be scrubbed.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`ghp_[A-Za-z0-9]{36,}`),
	regexp.MustCompile(`gho_[A-Za-z0-9]{36,}`),
	regexp.MustCompile(`sk-ant-[A-Za-z0-9\-]{20,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`GITHUB_TOKEN=[^\s]+`),
	regexp.MustCompile(`ANTHROPIC_API_KEY=[^\s]+`),
}

// ScrubSecrets removes secrets from all files in a directory.
func ScrubSecrets(dir string) {
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip git internals and binary files
		if strings.Contains(path, ".git/") {
			return nil
		}

		ext := filepath.Ext(path)
		textExts := map[string]bool{
			".go": true, ".py": true, ".ts": true, ".js": true, ".md": true,
			".txt": true, ".yaml": true, ".yml": true, ".json": true, ".toml": true,
			".env": true, ".sh": true, ".bash": true, ".zsh": true, ".cfg": true,
			".ini": true, ".conf": true, ".html": true, ".css": true, ".xml": true,
			".mod": true, ".sum": true, ".lock": true, ".tmpl": true, "": true,
		}
		if !textExts[ext] {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		content := string(data)
		modified := false
		for _, pat := range secretPatterns {
			if pat.MatchString(content) {
				content = pat.ReplaceAllString(content, "REDACTED")
				modified = true
			}
		}

		if modified {
			_ = os.WriteFile(path, []byte(content), info.Mode())
		}
		return nil
	})
}
