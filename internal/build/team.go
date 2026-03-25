package build

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// TeamBuild uses Claude Code --agents to run architect+coder+tester in parallel.
func TeamBuild(claudeBinary, workDir, specContent, language, name string, maxTurns int) error {
	agents := fmt.Sprintf(`{
		"architect": {
			"description": "Plans implementation from SPEC.md",
			"prompt": "You are the architect for %s. Read SPEC.md with 7 research papers. Design how to combine ALL 7 techniques. Output a plan: files to create, what each does, how techniques connect. Do NOT write code."
		},
		"coder": {
			"description": "Writes code following the plan",
			"prompt": "You are the coder for %s. Follow the architect's plan. Write production code. Resolve all dependencies immediately after each import."
		},
		"tester": {
			"description": "Writes tests and ensures they pass",
			"prompt": "You are the tester for %s. Write 3+ test files with real assertions. Run make test. Fix failures. Do not stop until make test passes."
		}
	}`, name, name, name)

	prompt := fmt.Sprintf("Build %s from SPEC.md. Language: %s.\n\nUse Agent tool: spawn architect to plan, coder to implement, tester to verify.\nModule path: github.com/timholm/%s\nDo not stop until make test passes.", name, language, name)

	cmd := exec.Command(claudeBinary,
		"-p", prompt,
		"--model", "sonnet",
		"--max-turns", fmt.Sprint(maxTurns),
		"--agents", agents,
		"--output-format", "text",
	)
	cmd.Dir = workDir
	cmd.Env = cleanTeamEnv()

	logFile, _ := os.OpenFile("/tmp/factory-v2.log", os.O_APPEND|os.O_WRONLY, 0644)
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		defer logFile.Close()
	}

	log.Printf("[team] building %s with architect+coder+tester (%d turns)", name, maxTurns)
	return cmd.Run()
}

func cleanTeamEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GITHUB_TOKEN=") || strings.HasPrefix(e, "ANTHROPIC_API_KEY=") {
			continue
		}
		env = append(env, e)
	}
	return env
}
