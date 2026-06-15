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

func TestWorktreeIsolator_Integrate合回并清理(t *testing.T) {
	repo := initRepo(t)
	w := &worktreeIsolator{repo: repo, label: "t"}
	dir, err := w.Setup()
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "out.txt"), []byte("产物"), 0o644)
	if err := w.SealSegment("impl"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Integrate(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "out.txt")); err != nil {
		t.Errorf("Integrate 后原仓库应有 out.txt: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Integrate 后 worktree 目录应已移除")
	}
	br, _ := gitIn(repo, "branch", "--list", w.branch)
	if strings.TrimSpace(string(br)) != "" {
		t.Errorf("Integrate 成功后应删临时分支, still: %q", br)
	}
}

func TestWorktreeIsolator_Abandon保留分支(t *testing.T) {
	repo := initRepo(t)
	w := &worktreeIsolator{repo: repo, label: "t"}
	dir, err := w.Setup()
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "out.txt"), []byte("产物"), 0o644)
	if err := w.SealSegment("impl"); err != nil {
		t.Fatal(err)
	}
	loc, err := w.Abandon()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(loc, w.branch) {
		t.Errorf("Abandon 应返回分支名, got %q", loc)
	}
	br, _ := gitIn(repo, "branch", "--list", w.branch)
	if strings.TrimSpace(string(br)) == "" {
		t.Errorf("Abandon 不应删分支")
	}
	show, _ := gitIn(repo, "show", w.branch+":out.txt")
	if strings.TrimSpace(string(show)) != "产物" {
		t.Errorf("分支应保有成果, got %q", show)
	}
}

func TestCopydirIsolator_拷出拷回不删(t *testing.T) {
	src := t.TempDir()
	_ = os.WriteFile(filepath.Join(src, "keep.txt"), []byte("原始"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "gone.txt"), []byte("将被删"), 0o644)

	c := &copydirIsolator{src: src}
	dir, err := c.Setup()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Errorf("快照应含 keep.txt: %v", err)
	}
	if err := c.SealSegment("x"); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "new.txt"), []byte("新增"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("改过"), 0o644)
	_ = os.Remove(filepath.Join(dir, "gone.txt"))

	if _, err := c.Integrate(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(src, "new.txt")); string(b) != "新增" {
		t.Errorf("新增文件应拷回, got %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(src, "keep.txt")); string(b) != "改过" {
		t.Errorf("修改应拷回, got %q", b)
	}
	if _, err := os.Stat(filepath.Join(src, "gone.txt")); err != nil {
		t.Errorf("删除不应镜像，gone.txt 应仍在: %v", err)
	}
}

func TestCopydirIsolator_Abandon保留副本(t *testing.T) {
	src := t.TempDir()
	_ = os.WriteFile(filepath.Join(src, "a.txt"), []byte("x"), 0o644)
	c := &copydirIsolator{src: src}
	dir, err := c.Setup()
	if err != nil {
		t.Fatal(err)
	}
	loc, err := c.Abandon()
	if err != nil {
		t.Fatal(err)
	}
	if loc != dir {
		t.Errorf("Abandon 应返回副本目录, got %q want %q", loc, dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); err != nil {
		t.Errorf("Abandon 应保留副本: %v", err)
	}
}

func TestWorktreeIsolator_脏标签也能建worktree(t *testing.T) {
	repo := initRepo(t)
	iso, err := newIsolator(repo, "plan impl/review!")
	if err != nil {
		t.Fatal(err)
	}
	w := iso.(*worktreeIsolator)
	if _, err := w.Setup(); err != nil {
		t.Fatalf("脏标签经 sanitize 后应能建 worktree: %v", err)
	}
	// 分支名只允许 ccr-chain/ 前缀里的 '/'；label 部分经 sanitize 不应带空格/'!'。
	if !strings.Contains(w.branch, "ccr-chain/plan-impl-review-") {
		t.Errorf("分支名应基于 sanitize 后的 label: %q", w.branch)
	}
}
