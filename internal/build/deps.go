package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// commonGoDeps maps import paths to go get targets.
var commonGoDeps = []string{
	"github.com/spf13/cobra@latest",
	"github.com/spf13/viper@latest",
	"gopkg.in/yaml.v3@latest",
	"github.com/google/uuid@latest",
	"github.com/jackc/pgx/v5@latest",
}

// ResolveDeps scans source files and fetches missing dependencies.
func ResolveDeps(workDir, language string) error {
	switch language {
	case "go", "":
		return resolveGoDeps(workDir)
	case "python":
		return resolvePythonDeps(workDir)
	case "typescript", "ts":
		return resolveTSDeps(workDir)
	}
	return nil
}

func resolveGoDeps(workDir string) error {
	// Read all .go files to find imports
	goContent := collectGoSource(workDir)

	for _, dep := range commonGoDeps {
		// Extract the import path (without @version)
		importPath := strings.Split(dep, "@")[0]
		if strings.Contains(goContent, importPath) {
			cmd := exec.Command("go", "get", dep)
			cmd.Dir = workDir
			_ = cmd.Run()
		}
	}

	// Always run go mod tidy
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = workDir
	return cmd.Run()
}

func collectGoSource(dir string) string {
	var sb strings.Builder
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		sb.Write(data)
		return nil
	})
	return sb.String()
}

func resolvePythonDeps(workDir string) error {
	if _, err := os.Stat(filepath.Join(workDir, "requirements.txt")); err == nil {
		cmd := exec.Command("pip", "install", "-r", "requirements.txt")
		cmd.Dir = workDir
		return cmd.Run()
	}
	return nil
}

func resolveTSDeps(workDir string) error {
	if _, err := os.Stat(filepath.Join(workDir, "package.json")); err == nil {
		cmd := exec.Command("npm", "install")
		cmd.Dir = workDir
		return cmd.Run()
	}
	return nil
}
