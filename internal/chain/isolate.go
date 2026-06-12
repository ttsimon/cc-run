package chain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Isolator 抽象「链的工作目录隔离 + 成果交回」策略：
// git 仓库用 worktree 隔离，非 git 目录用副本目录隔离。
type Isolator interface {
	// Setup 准备隔离工作目录，返回其路径。
	Setup() (string, error)
	// SealSegment 在某段执行完后封存该段产物（如打分段提交）。
	SealSegment(name string) error
	// Integrate 把隔离区成果交回原处，返回交回摘要。
	Integrate() (summary string, err error)
	// Abandon 放弃隔离区（不交回），返回成果保留位置。
	Abandon() (location string, err error)
}

// newIsolator 按 workdir 是否在 git 工作树内选择隔离策略。
func newIsolator(workdir, label string) (Isolator, error) {
	if isInsideWorkTree(workdir) {
		return &worktreeIsolator{repo: workdir, label: label}, nil
	}
	return &copydirIsolator{src: workdir}, nil
}

// isInsideWorkTree 判断 dir 是否处于 git 工作树内。
func isInsideWorkTree(dir string) bool {
	out, err := gitIn(dir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// CreateWorktree 在 repo 上开一个临时 git worktree（新分支），返回路径与清理函数。
// 链跑完/出错都应调 cleanup：移除 worktree 并删临时分支，原仓库不受影响。
func CreateWorktree(repo, label string) (dir string, cleanup func(), err error) {
	branch := fmt.Sprintf("ccr-chain/%s-%d", label, time.Now().Unix())
	dir = filepath.Join(os.TempDir(), fmt.Sprintf("ccr-chain-%d", time.Now().UnixNano()))

	if out, e := gitIn(repo, "worktree", "add", "-b", branch, dir); e != nil {
		return "", nil, fmt.Errorf("建 worktree 失败: %v %s", e, out)
	}
	cleanup = func() {
		_, _ = gitIn(repo, "worktree", "remove", "--force", dir)
		_, _ = gitIn(repo, "branch", "-D", branch)
	}
	return dir, cleanup, nil
}

func gitIn(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// sanitize 把 name 里的非字母数字换成 '-'，空名回退 "chain"。用于临时分支名。
func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "chain"
	}
	return b.String()
}

// worktreeIsolator 用临时 git worktree + 分段提交隔离 git 仓库工作目录。
type worktreeIsolator struct {
	repo   string
	label  string
	branch string
	dir    string
}

func (w *worktreeIsolator) Setup() (string, error)        { panic("not implemented") }
func (w *worktreeIsolator) SealSegment(name string) error { panic("not implemented") }
func (w *worktreeIsolator) Integrate() (string, error)    { panic("not implemented") }
func (w *worktreeIsolator) Abandon() (string, error)      { panic("not implemented") }

// copydirIsolator 用副本目录隔离非 git 工作目录。
type copydirIsolator struct {
	src string
	dir string
}

func (c *copydirIsolator) Setup() (string, error)        { panic("not implemented") }
func (c *copydirIsolator) SealSegment(name string) error { panic("not implemented") }
func (c *copydirIsolator) Integrate() (string, error)    { panic("not implemented") }
func (c *copydirIsolator) Abandon() (string, error)      { panic("not implemented") }
