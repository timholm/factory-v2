package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// SelfModify lets the overseer edit factory-v2's own code, test it, and redeploy.
// The factory literally rewrites itself.
func SelfModify(issue string, fix string) error {
	log.Printf("[selfmod] attempting self-modification: %s", issue)

	// Save current state in case we need to rollback
	backup := exec.Command("git", "stash")
	backup.Dir = os.Getenv("HOME") + "/factory-v2"
	backup.Run()

	// Use Claude Code with a team of agents to implement the fix
	agents := `{
		"architect": {
			"description": "Plans the code change — reads existing code, designs the fix",
			"prompt": "You are the architect. Read the codebase, understand the issue, design the minimal fix. Do NOT write code yet — just plan."
		},
		"coder": {
			"description": "Implements the planned fix",
			"prompt": "You are the coder. Implement the fix the architect planned. Write minimal, correct code. Run go build to verify."
		},
		"tester": {
			"description": "Verifies the fix works",
			"prompt": "You are the tester. Run go test ./... and verify everything passes. If tests fail, fix them."
		}
	}`

	prompt := fmt.Sprintf(`You are modifying the factory-v2 codebase to fix this issue:

ISSUE: %s
SUGGESTED FIX: %s

This is the factory's own code at /Users/tim/factory-v2.
Use the Agent tool to delegate:
1. Have the architect agent plan the change
2. Have the coder agent implement it
3. Have the tester agent verify it passes

Rules:
- Minimal changes only — don't refactor, don't add features
- Must compile: go build -buildvcs=false ./...
- Must pass tests: go test -buildvcs=false ./...
- Commit with message describing the fix
- Push to origin main

After pushing, the factory will auto-restart with the new code.`, issue, fix)

	cmd := exec.Command("claude",
		"-p", prompt,
		"--model", "opus",
		"--max-turns", "20",
		"--agents", agents,
		"--output-format", "text",
		"--dangerously-skip-permissions",
	)
	cmd.Dir = os.Getenv("HOME") + "/factory-v2"
	cmd.Env = cleanSelfModEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("[selfmod] modification failed: %v — rolling back", err)
		rollback := exec.Command("git", "stash", "pop")
		rollback.Dir = os.Getenv("HOME") + "/factory-v2"
		rollback.Run()
		return fmt.Errorf("selfmod failed: %w", err)
	}

	log.Printf("[selfmod] code change pushed — factory will restart with new code")

	// Rebuild binary
	build := exec.Command("go", "build", "-buildvcs=false", "-o", "factory-v3", ".")
	build.Dir = os.Getenv("HOME") + "/factory-v2"
	if out, err := build.CombinedOutput(); err != nil {
		log.Printf("[selfmod] rebuild failed: %s — rolling back", string(out))
		exec.Command("git", "revert", "--no-edit", "HEAD").Run()
		return fmt.Errorf("rebuild failed: %w", err)
	}

	// Restart ourselves
	log.Printf("[selfmod] restarting factory with new code...")
	restart := exec.Command("launchctl", "kickstart", "-k", "gui/"+fmt.Sprint(os.Getuid())+"/com.factory.v2")
	restart.Run()

	return nil
}

func cleanSelfModEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GITHUB_TOKEN=") || strings.HasPrefix(e, "ANTHROPIC_API_KEY=") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// TeamBuild uses Claude Code's --agents to run a team of specialists on each build.
// architect plans → coder implements → tester verifies → reviewer checks quality
func TeamBuild(claudeBinary, workDir, specContent, language, name string, maxTurns int) error {
	agents := fmt.Sprintf(`{
		"architect": {
			"description": "Plans implementation from SPEC.md — reads papers, designs architecture, outputs a plan",
			"prompt": "You are the architect for %s. Read SPEC.md. It has 7 research papers. Design how to combine ALL 7 techniques into one tool. Output a concrete implementation plan: which files to create, what each does, how the 7 techniques connect. Do NOT write code."
		},
		"coder": {
			"description": "Writes the actual code following the architect's plan",
			"prompt": "You are the coder for %s. Follow the architect's plan. Write production Go/Python code. After each file, run go build or python -c 'import X' to verify. Resolve all dependencies immediately."
		},
		"tester": {
			"description": "Writes tests and ensures they pass",
			"prompt": "You are the tester for %s. Write at least 3 test files with real assertions. Run make test. If tests fail, fix them. Do not stop until make test passes with >0 tests collected."
		}
	}`, name, name, name)

	prompt := fmt.Sprintf(`Build %s from SPEC.md. Language: %s.

Use the Agent tool to coordinate:
1. Spawn architect to plan the implementation
2. Spawn coder to write the code
3. Spawn tester to write and verify tests
4. Write README.md with references to all 7 papers
5. Run make test one final time

Module path: github.com/timholm/%s
Do not stop until make test passes.`, name, language, name)

	cmd := exec.Command(claudeBinary,
		"-p", prompt,
		"--model", "sonnet",
		"--max-turns", fmt.Sprint(maxTurns),
		"--agents", agents,
		"--output-format", "text",
	)
	cmd.Dir = workDir
	cmd.Env = cleanSelfModEnv()

	// Log output
	logFile, _ := os.OpenFile("/tmp/factory-v2.log", os.O_APPEND|os.O_WRONLY, 0644)
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		defer logFile.Close()
	}

	log.Printf("[team] building %s with architect+coder+tester team (%d turns)", name, maxTurns)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("team build: %w", err)
	}

	return nil
}

// WrapWithRetry handles rate limits by waiting and retrying
func WrapWithRetry(fn func() error) error {
	for attempt := 1; attempt <= 5; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "usage limit") ||
			strings.Contains(errStr, "hit your limit") || strings.Contains(errStr, "429") {
			wait := time.Duration(attempt*5) * time.Minute
			log.Printf("[retry] rate limited (attempt %d/5) — waiting %s", attempt, wait)
			time.Sleep(wait)
			continue
		}

		return err // non-rate-limit error, don't retry
	}
	return fmt.Errorf("rate limited after 5 attempts")
}
