package chain

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	_ = os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hi"), 0o644)
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return dir
}

func TestWorktree_建后存在拆后消失(t *testing.T) {
	repo := initRepo(t)
	wt, cleanup, err := CreateWorktree(repo, "ccr-chain-test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wt, "f.txt")); err != nil {
		t.Errorf("worktree 里应有仓库文件: %v", err)
	}
	cleanup()
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("cleanup 后 worktree 应被移除")
	}
}

func TestSanitize(t *testing.T) {
	if got := sanitize("plan impl/review!"); got != "plan-impl-review-" {
		t.Errorf("sanitize got %q", got)
	}
	if sanitize("") != "chain" {
		t.Error("空名应回退 chain")
	}
}
