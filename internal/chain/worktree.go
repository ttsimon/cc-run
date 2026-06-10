package chain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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
