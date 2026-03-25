package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// GitInit initializes a git repo in the work directory.
func GitInit(workDir string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = workDir
	cmd.Env = gitEnv()
	return cmd.Run()
}

// GitCommit stages all files and commits.
func GitCommit(workDir, message string) error {
	add := exec.Command("git", "add", "-A")
	add.Dir = workDir
	add.Env = gitEnv()
	if err := add.Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	commit := exec.Command("git", "commit", "-m", message)
	commit.Dir = workDir
	commit.Env = gitEnv()
	return commit.Run()
}

// GitMirrorToBare creates a bare repo and pushes to it.
func GitMirrorToBare(workDir, bareDir string) error {
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		return err
	}

	// Init bare repo
	initBare := exec.Command("git", "init", "--bare", bareDir)
	initBare.Env = gitEnv()
	if err := initBare.Run(); err != nil {
		return fmt.Errorf("init bare: %w", err)
	}

	// Push from work dir to bare repo
	push := exec.Command("git", "push", bareDir, "HEAD:refs/heads/main")
	push.Dir = workDir
	push.Env = gitEnv()
	return push.Run()
}

// GitPushToGitHub creates the repo on GitHub and pushes.
func GitPushToGitHub(ctx context.Context, workDir, githubUser, repoName, token string) error {
	// Create repo on GitHub (ignore error if exists)
	create := exec.CommandContext(ctx, "gh", "repo", "create",
		fmt.Sprintf("%s/%s", githubUser, repoName),
		"--public", "--source", workDir, "--push",
	)
	create.Dir = workDir
	create.Env = append(gitEnv(), fmt.Sprintf("GH_TOKEN=%s", token))

	if out, err := create.CombinedOutput(); err != nil {
		// If repo already exists, just add remote and push
		remote := exec.Command("git", "remote", "add", "origin",
			fmt.Sprintf("https://github.com/%s/%s.git", githubUser, repoName))
		remote.Dir = workDir
		remote.Env = gitEnv()
		_ = remote.Run() // ignore error if remote exists

		push := exec.CommandContext(ctx, "git", "push", "-u", "origin", "main", "--force")
		push.Dir = workDir
		push.Env = append(gitEnv(), fmt.Sprintf("GH_TOKEN=%s", token))
		if pushOut, pushErr := push.CombinedOutput(); pushErr != nil {
			return fmt.Errorf("push failed: %s\ncreate output: %s", string(pushOut), string(out))
		}
	}

	return nil
}

func gitEnv() []string {
	env := os.Environ()
	env = append(env,
		"GIT_AUTHOR_NAME=Factory",
		"GIT_AUTHOR_EMAIL=factory@localhost",
		"GIT_COMMITTER_NAME=Factory",
		"GIT_COMMITTER_EMAIL=factory@localhost",
	)
	return env
}
