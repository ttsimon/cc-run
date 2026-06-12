package chain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewIsolator_git目录选worktree(t *testing.T) {
	repo := initRepo(t)
	iso, err := newIsolator(repo, "t")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := iso.(*worktreeIsolator); !ok {
		t.Errorf("git 目录应选 worktreeIsolator, got %T", iso)
	}
}

func TestNewIsolator_非git目录选copydir(t *testing.T) {
	dir := t.TempDir()
	iso, err := newIsolator(dir, "t")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := iso.(*copydirIsolator); !ok {
		t.Errorf("非 git 目录应选 copydirIsolator, got %T", iso)
	}
}

func TestWorktreeIsolator_SetupAndSeal(t *testing.T) {
	repo := initRepo(t)
	w := &worktreeIsolator{repo: repo, label: "t"}
	dir, err := w.Setup()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "f.txt")); err != nil {
		t.Errorf("worktree 应含仓库文件: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".ccr-chain", ".gitignore")); err != nil {
		t.Errorf("应写入 .ccr-chain/.gitignore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "out.txt"), []byte("产物"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := w.SealSegment("impl"); err != nil {
		t.Fatal(err)
	}
	out, _ := gitIn(dir, "status", "--porcelain")
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("Seal 后应无残留, got %q", out)
	}
	log, _ := gitIn(dir, "show", "--name-only", "--format=", "HEAD")
	if !strings.Contains(string(log), "out.txt") {
		t.Errorf("提交应含 out.txt: %q", log)
	}
	if strings.Contains(string(log), ".ccr-chain") {
		t.Errorf("提交不应含 .ccr-chain: %q", log)
	}
	before, _ := gitIn(dir, "rev-list", "--count", "HEAD")
	if err := w.SealSegment("noop"); err != nil {
		t.Fatal(err)
	}
	after, _ := gitIn(dir, "rev-list", "--count", "HEAD")
	if strings.TrimSpace(string(before)) != strings.TrimSpace(string(after)) {
		t.Errorf("无残留不应补提交: %s -> %s", before, after)
	}
}
