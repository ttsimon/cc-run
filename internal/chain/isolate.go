package chain

import (
	"bytes"
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
		return &worktreeIsolator{repo: workdir, label: sanitize(label)}, nil
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

// gitIn 在 dir 跑 git，并用 GIT_CEILING_DIRECTORIES 把 git 锁死在 dir 内：
// 非 git 目录不会被父仓库的 .git 牵连（fix: chain 在父级是 git 的非 git 目录里
// segmentDiffStat / isInsideWorkTree 误指父仓库），worktreeIsolator 也被强制要求
// w.repo 自己就是 git 工作树根（不是的话 worktree add 会失败而非悄悄爬到根）。
//
// ceiling 必须是 dir 的**父目录**：git 文档语义是"不能进入这些目录"——把 dir 的
// 父级列为禁区，git 从 dir 向上找 .git 时进不了 parent 就停。EvalSymlinks 必须做：
// macOS 上 /var/folders/... 是 /private/var/folders/... 的软链，git 内部 realpath
// 当前路径后跟字面 ceiling 比对，不解析就匹不上、ceiling 失效。
func gitIn(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	canon := canonPath(dir)
	if canon != "" {
		parent := filepath.Dir(canon)
		// 已到根（filepath.Dir("/") == "/"）就不设 ceiling——本来就没父级可禁。
		if parent != canon {
			cmd.Env = append(os.Environ(), "GIT_CEILING_DIRECTORIES="+parent)
		}
	}
	return cmd.CombinedOutput()
}

// canonPath 转绝对 + 解析软链；路径不存在时**递归**向上找最深的存在祖先解析，剩余
// 部分原样拼回。这样 /var/cache/foo（中间 /var/cache 不存在但 /var 是软链）也能正确
// 解到 /private/var/cache/foo——否则 PathEscapes 会把同一物理路径的两种形式当成两条。
//
// 用例驱动：PathEscapes 要比的常是 Write 还没创建的新文件 / 系统未建的子目录。
func canonPath(p string) string {
	if p == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return canonWalk(abs)
}

// canonWalk 在已绝对化的路径上爬：能 EvalSymlinks 整段就用整段，否则切掉 base 递归。
func canonWalk(abs string) string {
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return abs // 到根（或自指），无法再往上
	}
	return filepath.Join(canonWalk(parent), filepath.Base(abs))
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

func (w *worktreeIsolator) Setup() (string, error) {
	w.branch = fmt.Sprintf("ccr-chain/%s-%d", w.label, time.Now().Unix())
	w.dir = filepath.Join(os.TempDir(), fmt.Sprintf("ccr-chain-%d", time.Now().UnixNano()))
	if out, err := gitIn(w.repo, "worktree", "add", "-b", w.branch, w.dir); err != nil {
		return "", fmt.Errorf("建 worktree 失败: %v %s", err, out)
	}
	// 运行产物目录不进提交：在 worktree 里忽略 .ccr-chain/。
	cdir := filepath.Join(w.dir, ".ccr-chain")
	if err := os.MkdirAll(cdir, 0o755); err == nil {
		_ = os.WriteFile(filepath.Join(cdir, ".gitignore"), []byte("*\n"), 0o644)
	}
	return w.dir, nil
}

func (w *worktreeIsolator) SealSegment(name string) error {
	out, err := gitIn(w.dir, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("查 worktree 状态失败: %v %s", err, out)
	}
	if strings.TrimSpace(string(out)) == "" {
		return nil // agent 已自行提交（或无改动），无残留
	}
	if out, err := gitIn(w.dir, "add", "-A"); err != nil {
		return fmt.Errorf("git add 失败: %v %s", err, out)
	}
	if out, err := gitIn(w.dir, "commit", "-m", "[ccr chain] "+name); err != nil {
		return fmt.Errorf("git commit 失败: %v %s", err, out)
	}
	return nil
}
func (w *worktreeIsolator) Integrate() (string, error) {
	// 先 ff-only；非 fast-forward 再尝试普通 merge；都失败则 abort 并报错（上层转 Abandon）。
	if out, err := gitIn(w.repo, "merge", "--ff-only", w.branch); err != nil {
		if out2, err2 := gitIn(w.repo, "merge", "--no-edit", w.branch); err2 != nil {
			_, _ = gitIn(w.repo, "merge", "--abort")
			return "", fmt.Errorf("合并成果失败（已 abort）: %v %s / %v %s", err, out, err2, out2)
		}
	}
	// 成果已在用户分支，删临时目录与分支都不丢东西。
	_, _ = gitIn(w.repo, "worktree", "remove", "--force", w.dir)
	_, _ = gitIn(w.repo, "branch", "-D", w.branch)
	return fmt.Sprintf("成果已合入当前分支（来自 %s）", w.branch), nil
}

func (w *worktreeIsolator) Abandon() (string, error) {
	// 只移除临时目录，绝不删分支——提交都在分支上，成果可取回。
	_, _ = gitIn(w.repo, "worktree", "remove", "--force", w.dir)
	return fmt.Sprintf("分支 %s（git merge %s 取回 / 不要可 git branch -D %s）", w.branch, w.branch, w.branch), nil
}

// copydirIsolator 用副本目录隔离非 git 工作目录。
type copydirIsolator struct {
	src string
	dir string
}

func (c *copydirIsolator) Setup() (string, error) {
	c.dir = filepath.Join(os.TempDir(), fmt.Sprintf("ccr-chain-%d", time.Now().UnixNano()))
	if err := copyTree(c.src, c.dir); err != nil {
		return "", fmt.Errorf("拷贝工作目录失败: %w", err)
	}
	return c.dir, nil
}

// SealSegment 对 copydir 无操作——文件就在副本里持久存在，天然不丢。
func (c *copydirIsolator) SealSegment(string) error { return nil }

func (c *copydirIsolator) Integrate() (string, error) {
	n, err := copyChangedBack(c.dir, c.src)
	if err != nil {
		return "", fmt.Errorf("拷回成果失败: %w", err)
	}
	return fmt.Sprintf("成果已拷回原目录（%d 个文件）", n), nil
}

// Abandon 保留副本、不拷回、不删，返回副本路径供用户自行取用。
func (c *copydirIsolator) Abandon() (string, error) {
	return c.dir, nil
}

// copyTree 递归把 src 拷到 dst，跳过 .git 与 .ccr-chain。
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		top := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
		if top == ".git" || top == ".ccr-chain" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(p, target)
	})
}

// copyChangedBack 把 tmp 里相对 src 新增/改动的文件拷回 src；不镜像删除。返回拷回数。
func copyChangedBack(tmp, src string) (int, error) {
	count := 0
	err := filepath.WalkDir(tmp, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(tmp, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		top := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
		if top == ".git" || top == ".ccr-chain" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		target := filepath.Join(src, rel)
		if sameContent(p, target) {
			return nil
		}
		if err := copyFile(p, target); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(src); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(dst, data, mode)
}

// sameContent 报告两文件内容是否一致；任一读失败视为不一致（宁可拷回）。
func sameContent(a, b string) bool {
	da, err := os.ReadFile(a)
	if err != nil {
		return false
	}
	db, err := os.ReadFile(b)
	if err != nil {
		return false
	}
	return bytes.Equal(da, db)
}
