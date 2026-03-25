package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Overseer is the outer layer that watches the factory like Tim would.
// It asks hard questions, finds misalignment with the vision, and forces fixes.
// Runs with Opus (highest thinking) every cycle.

const overseerPrompt = `You are Tim's proxy — a brutally honest overseer watching an autonomous software factory.

## Tim's Vision (non-negotiable)
1. Every product FUSES 7 DIFFERENT research paper techniques into ONE tool
2. Every product MUST improve the factory itself — infinite self-improvement loop
3. The factory must run 24/7 with ZERO human intervention
4. Repos must have REAL code, REAL tests, REAL references to all 7 papers
5. Quality over quantity — one excellent repo beats ten mediocre ones
6. Local repos are source of truth, GitHub is mirror
7. No sleeping, no stopping, always producing

## Current System Status
%s

## Your Job
You are NOT the factory. You are watching it from outside. Ask:

1. IS IT ACTUALLY RUNNING? Check the process list, the logs, the timestamps. If the last log entry is more than 10 minutes old, something is stuck.

2. IS IT PRODUCING? Count repos shipped in the last hour. If zero, WHY? Don't accept excuses — find the root cause.

3. IS THE QUALITY REAL? For the most recent shipped repo:
   - Does it actually compile?
   - Does it have tests that pass?
   - Does it reference all 7 papers?
   - Does it improve the factory?
   - Would Tim look at this and be impressed or frustrated?

4. IS IT ALIGNED WITH THE VISION? The factory should be building tools that make itself better. Is it doing that, or is it building random IoT anomaly detection systems?

5. WHAT WOULD TIM SAY? If Tim looked at the dashboard right now, would he be frustrated? If yes, what specifically would frustrate him and how do you fix it?

## What You Can Do
Return a JSON object with:
{
  "status": "running|stuck|broken|idle",
  "frustration_level": 1-10,
  "what_tim_would_say": "the honest thing Tim would say looking at this",
  "issues": [
    {
      "problem": "specific problem",
      "evidence": "what you see that proves it",
      "fix": "a SINGLE executable shell command (not English description — a real bash command that fixes it)",
      "priority": "critical|high|medium"
    }
  ],
  "actions_taken": ["what you actually fixed"],
  "next_product_suggestion": "what the factory should build next to improve itself"
}

Be Tim. Be frustrated when things aren't working. Be specific. No hand-waving.`

func runOverseer() {
	// Log to both main log and overseer-specific log
	overseerLog, _ := os.OpenFile("/tmp/factory-overseer.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if overseerLog != nil {
		defer overseerLog.Close()
	}
	logBoth := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		log.Print(msg)
		if overseerLog != nil {
			fmt.Fprintf(overseerLog, "%s %s\n", time.Now().Format("2006/01/02 15:04:05"), msg)
		}
	}

	logBoth("[overseer] === OVERSEER AUDIT STARTING ===")

	status := collectOverseerStatus()
	prompt := fmt.Sprintf(overseerPrompt, status)

	// Call Opus with highest thinking
	// Pipe prompt via stdin with --print (no tools, just text response)
	ctx2, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx2, "claude", "--print", "--output-format", "text", "--model", "sonnet")
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Env = cleanOverseerEnv()
	cmd.Dir = os.TempDir()

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[overseer] Opus call failed: %v", err)
		return
	}

	// Parse and act on the response
	response := string(out)
	log.Printf("[overseer] Opus says:\n%s", response)

	// Try to parse JSON from response
	var audit OverseerAudit
	jsonStr := extractJSON(response)
	if err := json.Unmarshal([]byte(jsonStr), &audit); err == nil {
		log.Printf("[overseer] Status: %s | Frustration: %d/10", audit.Status, audit.FrustrationLevel)
		log.Printf("[overseer] Tim would say: %s", audit.WhatTimWouldSay)
		for _, issue := range audit.Issues {
			log.Printf("[overseer] [%s] %s — %s", issue.Priority, issue.Problem, issue.Fix)
		}
		if audit.NextProductSuggestion != "" {
			log.Printf("[overseer] Next product suggestion: %s", audit.NextProductSuggestion)
		}

		// Auto-fix critical issues
		for _, issue := range audit.Issues {
			if issue.Priority == "critical" && issue.Fix != "" {
				// Try shell command first
				log.Printf("[overseer] AUTO-FIXING: %s", issue.Fix)
				fixCmd := exec.Command("sh", "-c", issue.Fix)
				fixOut, fixErr := fixCmd.CombinedOutput()
				if fixErr != nil {
					log.Printf("[overseer] shell fix failed, trying self-modification: %v", fixErr)
					// Use Claude Code team to fix the factory's own code
					if err := SelfModify(issue.Problem, issue.Fix); err != nil {
						log.Printf("[overseer] self-modification failed: %v", err)
					}
				} else {
					log.Printf("[overseer] fix applied: %s", string(fixOut))
				}
			}
		}
	}

	log.Println("[overseer] === OVERSEER AUDIT COMPLETE ===")
}

type OverseerAudit struct {
	Status                string          `json:"status"`
	FrustrationLevel      int             `json:"frustration_level"`
	WhatTimWouldSay       string          `json:"what_tim_would_say"`
	Issues                []OverseerIssue `json:"issues"`
	ActionsTaken          []string        `json:"actions_taken"`
	NextProductSuggestion string          `json:"next_product_suggestion"`
}

type OverseerIssue struct {
	Problem  string `json:"problem"`
	Evidence string `json:"evidence"`
	Fix      string `json:"fix"`
	Priority string `json:"priority"`
}

func collectOverseerStatus() string {
	var sb strings.Builder

	// Process list
	sb.WriteString("## Running Processes\n")
	if out, err := exec.Command("sh", "-c", "ps aux | grep -E 'claude|factory-v2|go run' | grep -v grep").CombinedOutput(); err == nil {
		sb.WriteString(string(out))
	}

	// Last 30 lines of log
	sb.WriteString("\n## Factory Log (last 30 lines)\n")
	if out, err := exec.Command("tail", "-30", "/tmp/factory-v2.log").CombinedOutput(); err == nil {
		sb.WriteString(string(out))
	}

	// K8s pods
	sb.WriteString("\n## K8s Pods\n")
	if out, err := exec.Command("kubectl", "get", "pods", "-n", "factory", "--no-headers").CombinedOutput(); err == nil {
		sb.WriteString(string(out))
	}

	// GitHub repo count
	sb.WriteString("\n## GitHub\n")
	if out, err := exec.Command("gh", "repo", "list", "timholm", "--limit", "200", "--json", "name,isArchived", "--jq", `[.[] | select(.isArchived == false)] | length`).CombinedOutput(); err == nil {
		sb.WriteString(fmt.Sprintf("Active repos: %s", string(out)))
	}

	// Local bare repos
	sb.WriteString("\n## Local Bare Repos\n")
	if out, err := exec.Command("sh", "-c", "ls ~/factory-git/*.git 2>/dev/null | wc -l").CombinedOutput(); err == nil {
		sb.WriteString(fmt.Sprintf("Count: %s", string(out)))
	}

	// tmux sessions
	sb.WriteString("\n## Tmux Sessions\n")
	if out, err := exec.Command("tmux", "list-sessions").CombinedOutput(); err == nil {
		sb.WriteString(string(out))
	}

	// Last shipped repo
	sb.WriteString("\n## Last Shipped\n")
	if out, err := exec.Command("gh", "repo", "list", "timholm", "--limit", "3", "--json", "name,createdAt", "--jq", `sort_by(.createdAt) | reverse | .[:3] | .[] | "\(.createdAt) \(.name)"`).CombinedOutput(); err == nil {
		sb.WriteString(string(out))
	}

	// Current time
	sb.WriteString(fmt.Sprintf("\n## Current Time: %s\n", time.Now().Format(time.RFC3339)))

	return sb.String()
}

func cleanOverseerEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GITHUB_TOKEN=") || strings.HasPrefix(e, "ANTHROPIC_API_KEY=") {
			continue
		}
		env = append(env, e)
	}
	return env
}

func extractJSON(s string) string {
	// Find JSON object in response
	start := strings.Index(s, "{")
	if start < 0 {
		return "{}"
	}
	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return "{}"
}

// RunOverseerLoop runs the overseer on a timer alongside the factory.
func RunOverseerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			runOverseer()
			// Run continuously — finish one audit, immediately start the next
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}
}
