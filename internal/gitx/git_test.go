package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGitRefsCommitsStatus(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	gitCmd(t, repo, "init")
	gitCmd(t, repo, "config", "user.email", "agentdb233@example.test")
	gitCmd(t, repo, "config", "user.name", "agentdb233")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repo, "add", "README.md")
	gitCmd(t, repo, "commit", "-m", "initial")
	gitCmd(t, repo, "tag", "v0.1.0")
	gitCmd(t, repo, "checkout", "-b", "feature/search")

	refs, err := ListRefs(repo)
	if err != nil {
		t.Fatal(err)
	}
	if refs.Head != "feature/search" {
		t.Fatalf("head=%q", refs.Head)
	}
	if !contains(refs.Branches, "feature/search") || !contains(refs.Tags, "v0.1.0") {
		t.Fatalf("refs=%+v", refs)
	}
	commits, err := ListCommits(repo, "HEAD", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 || commits[0].Subject != "initial" {
		t.Fatalf("commits=%+v", commits)
	}
	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := WorktreeStatus(repo)
	if err != nil {
		t.Fatal(err)
	}
	if st.Clean || len(st.Changes) == 0 {
		t.Fatalf("status=%+v", st)
	}
}

func gitCmd(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if runtime.GOOS == "windows" {
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
