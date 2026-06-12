# ccr chain 隔离与成果交回（track A）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
> **提交约定**：Conventional Commits；每条 commit 信息末尾加 `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`。提交前过 `task check`（fmt+vet+lint+test）。

**Goal:** 重做 chain 隔离层，让链跑完的成果**永不被静默销毁**——成功合回用户仓库，失败/退出则保留并打印取回路径。

**Architecture:** 用可插拔 `Isolator` 接口（`Setup/SealSegment/Integrate/Abandon`）替换现在 orchestrate 里写死的 `CreateWorktree`+无条件 cleanup。git 目录用 `worktreeIsolator`（每段兜底提交、ff/普通 merge 合回、失败只删临时目录保留分支），非 git 目录用 `copydirIsolator`（拷贝快照、变更拷回、不镜像删除）。orchestrate 按「跑完+pass / needs-work / 报错或退出」三态分别 Integrate / Abandon。

**Tech Stack:** Go（CGO_ENABLED=0）、`os/exec` 调 git、标准库 `filepath.WalkDir`。spec 见 `docs/superpowers/specs/2026-06-11-ccr-chain-isolation-design.md`。

---

## File Structure

- **Create** `internal/chain/isolate.go` — `Isolator` 接口、`newIsolator` 选择器、`worktreeIsolator`、`copydirIsolator`、拷贝助手（`copyTree`/`copyChangedBack`/`copyFile`/`sameContent`），并吸收原 `worktree.go` 的 `gitIn`/`sanitize`。
- **Create** `internal/chain/isolate_test.go` — 两种 Isolator 与选择器的单测。
- **Delete** `internal/chain/worktree.go` — `CreateWorktree` 不再使用；`gitIn`/`sanitize` 迁入 `isolate.go`。
- **Modify** `internal/chain/worktree_test.go` — 删 `TestWorktree_建后存在拆后消失`，保留 `initRepo`（被 orchestrate_test 复用）与 `TestSanitize`。
- **Modify** `internal/chain/orchestrate.go` — 加 `Out io.Writer` 字段；`Run` 改用 Isolator + 三态收尾。
- **Modify** `internal/chain/orchestrate_test.go` — 加事故回归测试（成果不丢）。

每个文件单一职责：`isolate.go` 只管隔离与交回；orchestrate 只管编排与三态决策。

---

## Task 1: Isolator 接口 + 选择器，迁移 git/sanitize 助手

把隔离抽象立起来，并把 `worktree.go` 的工具函数搬到新文件，删掉旧的 `CreateWorktree`。

**Files:**
- Create: `internal/chain/isolate.go`
- Create: `internal/chain/isolate_test.go`
- Delete: `internal/chain/worktree.go`
- Modify: `internal/chain/worktree_test.go`（删一个测试）

- [ ] **Step 1: 写失败测试 — 选择器按上下文选实现**

在 `internal/chain/isolate_test.go`：

```go
package chain

import "testing"

func TestNewIsolator_git目录选worktree(t *testing.T) {
	repo := initRepo(t) // 复用 worktree_test.go 的 helper
	iso, err := newIsolator(repo, "t")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := iso.(*worktreeIsolator); !ok {
		t.Errorf("git 目录应选 worktreeIsolator, got %T", iso)
	}
}

func TestNewIsolator_非git目录选copydir(t *testing.T) {
	dir := t.TempDir() // 裸临时目录，非 git
	iso, err := newIsolator(dir, "t")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := iso.(*copydirIsolator); !ok {
		t.Errorf("非 git 目录应选 copydirIsolator, got %T", iso)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/chain/ -run TestNewIsolator -v`
Expected: 编译失败（`newIsolator`/`worktreeIsolator`/`copydirIsolator` 未定义）。

- [ ] **Step 3: 创建 isolate.go（接口 + 选择器 + 迁移助手 + 两个空壳类型）**

新建 `internal/chain/isolate.go`：

```go
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

// Isolator 把一条链的执行与用户真实目录隔离，并在结束时决定成果如何交回。
// 铁律：除 Integrate 成功外，任何路径都不得静默销毁成果。
type Isolator interface {
	Setup() (workdir string, err error)     // 准备隔离工作区，返回段应在其中运行的目录
	SealSegment(name string) error           // 一段跑完后把该段成果落成持久形态
	Integrate() (summary string, err error)  // 成功：把成果交回用户仓库
	Abandon() (location string, err error)   // 非成功：保留成果，返回取回位置
}

// newIsolator 按上下文选隔离实现：git 工作树→worktree，否则→copydir。
func newIsolator(workdir, label string) (Isolator, error) {
	if isInsideWorkTree(workdir) {
		return &worktreeIsolator{repo: workdir, label: label}, nil
	}
	return &copydirIsolator{src: workdir}, nil
}

func isInsideWorkTree(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// gitIn 在 dir 里跑 git，返回合并输出。（迁自原 worktree.go）
func gitIn(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// sanitize 把 name 里的非字母数字换成 '-'，空名回退 "chain"。用于临时分支名。（迁自原 worktree.go）
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

// --- worktreeIsolator（git 目录）---

type worktreeIsolator struct {
	repo   string // 用户原仓库
	label  string // 临时分支标签
	branch string // 本次临时分支名
	dir    string // 临时 worktree 目录
}

// --- copydirIsolator（非 git 目录）---

type copydirIsolator struct {
	src string // 用户原目录
	dir string // 临时副本目录
}

// 以下助手在后续任务实现，先放占位以便编译。
var _ = bytes.Equal
var _ = filepath.Join
var _ = fmt.Sprintf
var _ = os.MkdirAll
var _ = time.Now
```

> 注：`var _ = ...` 占位行仅为 Task 1 通过编译，**在 Task 2/4 写完真实实现后删除**。

- [ ] **Step 4: 删除 worktree.go，并从 worktree_test.go 删掉 CreateWorktree 测试**

删除整个文件 `internal/chain/worktree.go`。

在 `internal/chain/worktree_test.go` 删除 `TestWorktree_建后存在拆后消失` 整个函数（含 import 中变得未用的项由 gofmt/编译提示清理），保留 `initRepo` 与 `TestSanitize`。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/chain/ -run 'TestNewIsolator|TestSanitize' -v`
Expected: PASS（选择器命中正确类型；sanitize 仍工作）。

- [ ] **Step 6: 提交**

```bash
git add internal/chain/isolate.go internal/chain/isolate_test.go internal/chain/worktree_test.go
git rm internal/chain/worktree.go
git commit -m "refactor: introduce pluggable Isolator, retire CreateWorktree"
```

---

## Task 2: worktreeIsolator 的 Setup 与 SealSegment

git 目录：开临时 worktree、忽略 `.ccr-chain/`、每段有残留就兜底提交。

**Files:**
- Modify: `internal/chain/isolate.go`
- Modify: `internal/chain/isolate_test.go`

- [ ] **Step 1: 写失败测试 — Setup 建出 worktree，SealSegment 按残留补/不补提交**

追加到 `internal/chain/isolate_test.go`：

```go
import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeIsolator_SetupAndSeal(t *testing.T) {
	repo := initRepo(t)
	w := &worktreeIsolator{repo: repo, label: "t"}
	dir, err := w.Setup()
	if err != nil {
		t.Fatal(err)
	}
	// 原仓库文件应出现在 worktree
	if _, err := os.Stat(filepath.Join(dir, "f.txt")); err != nil {
		t.Errorf("worktree 应含仓库文件: %v", err)
	}
	// .ccr-chain 应被忽略
	if _, err := os.Stat(filepath.Join(dir, ".ccr-chain", ".gitignore")); err != nil {
		t.Errorf("应写入 .ccr-chain/.gitignore: %v", err)
	}

	// 模拟 agent 没提交就产出文件
	if err := os.WriteFile(filepath.Join(dir, "out.txt"), []byte("产物"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := w.SealSegment("impl"); err != nil {
		t.Fatal(err)
	}
	// 兜底提交后，worktree 应干净
	out, _ := gitIn(dir, "status", "--porcelain")
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("Seal 后应无残留, got %q", out)
	}
	// 提交里应有 out.txt，但不含 .ccr-chain
	log, _ := gitIn(dir, "show", "--name-only", "--format=", "HEAD")
	if !strings.Contains(string(log), "out.txt") {
		t.Errorf("提交应含 out.txt: %q", log)
	}
	if strings.Contains(string(log), ".ccr-chain") {
		t.Errorf("提交不应含 .ccr-chain: %q", log)
	}

	// 无残留时再 Seal 不应新增提交
	before, _ := gitIn(dir, "rev-list", "--count", "HEAD")
	if err := w.SealSegment("noop"); err != nil {
		t.Fatal(err)
	}
	after, _ := gitIn(dir, "rev-list", "--count", "HEAD")
	if strings.TrimSpace(string(before)) != strings.TrimSpace(string(after)) {
		t.Errorf("无残留不应补提交: %s -> %s", before, after)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/chain/ -run TestWorktreeIsolator_SetupAndSeal -v`
Expected: 编译/运行失败（`Setup`/`SealSegment` 方法未实现）。

- [ ] **Step 3: 实现 Setup 与 SealSegment**

在 `internal/chain/isolate.go` 的 worktreeIsolator 区块下加：

```go
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
```

删除 Task 1 里 isolate.go 末尾的 `var _ = ...` 占位中已被真实引用的项（`fmt`/`os`/`filepath`/`time` 现已用上；`bytes` 待 Task 4 用，暂留 `var _ = bytes.Equal`）。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/chain/ -run TestWorktreeIsolator_SetupAndSeal -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/chain/isolate.go internal/chain/isolate_test.go
git commit -m "feat: worktree isolator setup and per-segment seal commit"
```

---

## Task 3: worktreeIsolator 的 Integrate 与 Abandon

成功 ff/普通 merge 合回并删临时分支；失败/放弃只删临时目录、**保留分支**。

**Files:**
- Modify: `internal/chain/isolate.go`
- Modify: `internal/chain/isolate_test.go`

- [ ] **Step 1: 写失败测试 — Integrate 合回并清理；Abandon 保分支**

追加到 `internal/chain/isolate_test.go`：

```go
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
	// 成果应出现在原仓库工作树
	if _, err := os.Stat(filepath.Join(repo, "out.txt")); err != nil {
		t.Errorf("Integrate 后原仓库应有 out.txt: %v", err)
	}
	// 临时目录应被移除
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Integrate 后 worktree 目录应已移除")
	}
	// 临时分支应被删除
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
	// 分支仍在（成果可取回）
	br, _ := gitIn(repo, "branch", "--list", w.branch)
	if strings.TrimSpace(string(br)) == "" {
		t.Errorf("Abandon 不应删分支")
	}
	// 分支上确有 out.txt（提交未丢）
	show, _ := gitIn(repo, "show", w.branch+":out.txt")
	if strings.TrimSpace(string(show)) != "产物" {
		t.Errorf("分支应保有成果, got %q", show)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/chain/ -run 'TestWorktreeIsolator_Integrate|TestWorktreeIsolator_Abandon' -v`
Expected: 失败（`Integrate`/`Abandon` 未实现）。

- [ ] **Step 3: 实现 Integrate 与 Abandon**

在 `internal/chain/isolate.go` worktreeIsolator 区块加：

```go
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
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/chain/ -run 'TestWorktreeIsolator_Integrate|TestWorktreeIsolator_Abandon' -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/chain/isolate.go internal/chain/isolate_test.go
git commit -m "feat: worktree isolator integrate (merge back) and abandon (keep branch)"
```

---

## Task 4: copydirIsolator（非 git 目录）

拷贝快照、把新增/修改的文件拷回、不镜像删除；放弃则保留副本返回路径。

**Files:**
- Modify: `internal/chain/isolate.go`
- Modify: `internal/chain/isolate_test.go`

- [ ] **Step 1: 写失败测试 — 拷出/拷回/不删/放弃保留**

追加到 `internal/chain/isolate_test.go`：

```go
func TestCopydirIsolator_拷出拷回不删(t *testing.T) {
	src := t.TempDir()
	_ = os.WriteFile(filepath.Join(src, "keep.txt"), []byte("原始"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "gone.txt"), []byte("将被删"), 0o644)

	c := &copydirIsolator{src: src}
	dir, err := c.Setup()
	if err != nil {
		t.Fatal(err)
	}
	// 快照应含两文件
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Errorf("快照应含 keep.txt: %v", err)
	}
	// SealSegment 是 no-op
	if err := c.SealSegment("x"); err != nil {
		t.Fatal(err)
	}
	// 副本里新增 + 修改 + 删除
	_ = os.WriteFile(filepath.Join(dir, "new.txt"), []byte("新增"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("改过"), 0o644)
	_ = os.Remove(filepath.Join(dir, "gone.txt"))

	if _, err := c.Integrate(); err != nil {
		t.Fatal(err)
	}
	// 新增拷回
	if b, _ := os.ReadFile(filepath.Join(src, "new.txt")); string(b) != "新增" {
		t.Errorf("新增文件应拷回, got %q", b)
	}
	// 修改拷回
	if b, _ := os.ReadFile(filepath.Join(src, "keep.txt")); string(b) != "改过" {
		t.Errorf("修改应拷回, got %q", b)
	}
	// 删除不镜像：原目录 gone.txt 仍在
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
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/chain/ -run TestCopydirIsolator -v`
Expected: 失败（copydir 方法 + 拷贝助手未实现）。

- [ ] **Step 3: 实现 copydir 方法 + 拷贝助手**

在 `internal/chain/isolate.go` copydirIsolator 区块加，并删除 Task 1 残留的 `var _ = bytes.Equal` 占位（`bytes` 现由 `sameContent` 真正使用）：

```go
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
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/chain/ -run TestCopydirIsolator -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/chain/isolate.go internal/chain/isolate_test.go
git commit -m "feat: copydir isolator for non-git workdirs (copy snapshot, copy back, no delete mirror)"
```

---

## Task 5: orchestrate 接入 Isolator + 三态收尾

把 `Run` 从「写死 worktree + 无条件 cleanup」改为「按上下文建 Isolator、每段 Seal、三态 Integrate/Abandon」，并加事故回归测试。

**Files:**
- Modify: `internal/chain/orchestrate.go`
- Modify: `internal/chain/orchestrate_test.go`

- [ ] **Step 1: 写失败测试 — 成功合回、needs-work 保留、报错保留**

追加到 `internal/chain/orchestrate_test.go`（文件已 import `os`/`path/filepath`/`strings`/`testing`）：

```go
// 事故回归：agent 没提交就结束，成果绝不能丢——isolate 成功应把产物合回原仓库。
func TestOrchestrate_isolate成功合回成果(t *testing.T) {
	repo := initRepo(t)
	c := Chain{
		Name:     "t",
		Isolate:  true,
		Workdir:  repo,
		Segments: []Segment{{Name: "impl", Profile: "strong", Prompt: "x"}},
	}
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.Out = &strings.Builder{}
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		// 模拟 agent 写文件但不自己提交
		_ = os.WriteFile(filepath.Join(spec.Workdir, "out.txt"), []byte("产物"), 0o644)
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(repo, "out.txt")); string(b) != "产物" {
		t.Errorf("成功应把成果合回原仓库, got %q", b)
	}
}

func TestOrchestrate_isolate_needswork不合回(t *testing.T) {
	repo := initRepo(t)
	c := Chain{
		Name:    "t",
		Isolate: true,
		Workdir: repo,
		Segments: []Segment{
			{Name: "review", Profile: "strong", Prompt: "审", Review: true},
		},
	}
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.Out = &strings.Builder{}
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		_ = os.WriteFile(filepath.Join(spec.Workdir, "out.txt"), []byte("半成品"), 0o644)
		// 审查段写 needs-work
		cd := filepath.Join(spec.Workdir, ".ccr-chain")
		_ = os.MkdirAll(cd, 0o755)
		_ = os.WriteFile(filepath.Join(cd, "verdict"), []byte("needs-work"), 0o644)
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	// needs-work 不应自动合回
	if _, err := os.Stat(filepath.Join(repo, "out.txt")); !os.IsNotExist(err) {
		t.Errorf("needs-work 不应把成果合回原仓库")
	}
}

func TestOrchestrate_isolate_段报错保留成果(t *testing.T) {
	repo := initRepo(t)
	c := Chain{
		Name:     "t",
		Isolate:  true,
		Workdir:  repo,
		Segments: []Segment{{Name: "impl", Profile: "strong", Prompt: "x"}},
	}
	o := NewOrchestrator(testReg())
	o.Auto = true
	out := &strings.Builder{}
	o.Out = out
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		_ = os.WriteFile(filepath.Join(spec.Workdir, "out.txt"), []byte("产物"), 0o644)
		return "", 7, nil // 非 0 退出
	}
	if err := o.Run(c); err == nil {
		t.Fatal("段非 0 退出应报错")
	}
	// 报错路径应打印保留位置，且不静默销毁
	if !strings.Contains(out.String(), "保留") {
		t.Errorf("报错应打印成果保留位置, got %q", out.String())
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/chain/ -run 'TestOrchestrate_isolate成功|TestOrchestrate_isolate_' -v`
Expected: 失败（`o.Out` 字段不存在 / Integrate 未接入）。

- [ ] **Step 3: 给 Orchestrator 加 Out 字段并重写 Run**

修改 `internal/chain/orchestrate.go`。先在 import 加 `"io"`；给结构体加字段：

```go
type Orchestrator struct {
	reg    *registry.Registry
	Auto   bool
	Input  string
	Pauser Pauser
	Out    io.Writer // 隔离结果/进度输出；nil 默认 os.Stdout

	runSegment func(spec runSpec, seg Segment) (string, int, error)
}
```

在 `NewOrchestrator` 末尾设默认：

```go
	o.Pauser = NewTermPauser()
	o.Out = os.Stdout
	return o
```

把 `Run` 整体替换为：

```go
func (o *Orchestrator) Run(c Chain) error {
	out := o.Out
	if out == nil {
		out = os.Stdout
	}
	workdir := c.Workdir
	if workdir == "" {
		workdir = "."
	}

	var iso Isolator
	if c.Isolate {
		var err error
		iso, err = newIsolator(workdir, sanitize(c.Name))
		if err != nil {
			return fmt.Errorf("建隔离区失败: %w", err)
		}
		wd, err := iso.Setup()
		if err != nil {
			return fmt.Errorf("隔离区 Setup 失败: %w", err)
		}
		workdir = wd
	}

	ccrPath := "ccr"
	if exe, err := os.Executable(); err == nil {
		ccrPath = exe
	}

	needsWork := false
	var prev string
	for i := 0; i < len(c.Segments); i++ {
		seg := c.Segments[i]

		p, err := o.reg.Resolve(seg.Profile)
		if err != nil {
			abandon(out, iso)
			return fmt.Errorf("段 #%d(%q) 的 profile 解析失败: %w", i, seg.Name, err)
		}
		renderedPrompt := Render(seg.Prompt, prev, o.Input)
		if seg.Review {
			renderedPrompt += ReviewInstruction()
		}
		env := map[string]string{}
		for k, v := range p.Env {
			env[k] = v
		}
		env["CCR_CHAIN_DENY"] = strings.Join(MergeDenylist(DefaultDenylist(), seg.DenyCommands), "\n")

		settingsPath := ""
		settingsDir := filepath.Join(workdir, ".ccr-chain")
		if err := os.MkdirAll(settingsDir, 0o755); err == nil {
			settingsPath = filepath.Join(settingsDir, "settings-"+sanitize(seg.Name)+".json")
			_ = os.WriteFile(settingsPath, []byte(SettingsJSON(ccrPath)), 0o644)
		}

		spec := runSpec{
			Prompt:       renderedPrompt,
			AllowTools:   seg.AllowTools,
			Workdir:      workdir,
			SettingsPath: settingsPath,
			Env:          env,
		}
		segOut, code, err := o.runSegment(spec, seg)
		if err != nil {
			abandon(out, iso)
			return fmt.Errorf("段 #%d(%q) 启动失败: %w", i, seg.Name, err)
		}
		if code != 0 {
			abandon(out, iso)
			return fmt.Errorf("段 #%d(%q) 非 0 退出（%d），中止", i, seg.Name, code)
		}
		prev = segOut

		// 段成果落成持久形态（worktree 兜底提交 / copydir 无操作）。
		if iso != nil {
			if err := iso.SealSegment(seg.Name); err != nil {
				abandon(out, iso)
				return fmt.Errorf("段 #%d(%q) 成果固化失败: %w", i, seg.Name, err)
			}
		}
		if seg.Review && ReadVerdict(workdir) == VerdictNeedsWork {
			needsWork = true
		}

		// 放行点（非 Auto，且后面还有段）
		if !o.Auto && i+1 < len(c.Segments) {
			info := prev
			if seg.Review {
				switch ReadVerdict(workdir) {
				case VerdictPass:
					info += "\n[判定] pass ✓"
				case VerdictNeedsWork:
					info += "\n[判定] needs-work —— 下一段建议放行修复"
				}
			}
			next := c.Segments[i+1]
			d, edited, perr := o.Pauser.Pause(next, info)
			if perr != nil {
				abandon(out, iso)
				return perr
			}
			switch d {
			case DecisionQuit:
				if iso != nil {
					loc, _ := iso.Abandon()
					fmt.Fprintf(out, "已退出，成果保留在 %s\n", loc)
				}
				return nil
			case DecisionSkip:
				i++
			case DecisionEdit:
				if edited != "" {
					c.Segments[i+1].Prompt = edited
				}
			case DecisionProceed:
			}
		}
	}

	// 结束三态：跑完+pass→Integrate；needs-work→Abandon；（报错/退出已在上面 Abandon）
	if iso != nil {
		if needsWork {
			loc, _ := iso.Abandon()
			fmt.Fprintf(out, "审查判定 needs-work，成果未自动合入，保留在 %s\n", loc)
		} else {
			summary, err := iso.Integrate()
			if err != nil {
				loc, _ := iso.Abandon()
				fmt.Fprintf(out, "合并失败，成果保留在 %s（%v）\n", loc, err)
			} else {
				fmt.Fprintln(out, summary)
			}
		}
	}
	return nil
}

// abandon 在异常路径保留成果并打印取回位置（iso 为 nil 时无操作）。
func abandon(out io.Writer, iso Isolator) {
	if iso == nil {
		return
	}
	loc, _ := iso.Abandon()
	fmt.Fprintf(out, "成果保留在 %s（未合入）\n", loc)
}
```

- [ ] **Step 4: 运行三态测试确认通过**

Run: `go test ./internal/chain/ -run 'TestOrchestrate_isolate' -v`
Expected: PASS（含既有 `TestOrchestrate_isolate跑在worktree` —— 单段无改动 → Integrate ff no-op → worktree 移除）。

- [ ] **Step 5: 跑整包回归**

Run: `go test ./internal/chain/ -v`
Expected: 全 PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/chain/orchestrate.go internal/chain/orchestrate_test.go
git commit -m "feat: orchestrate three-state result handback via Isolator, never silently destroy"
```

---

## Task 6: 文档同步 + 全套校验

把隔离新语义写进 spec/设计文档，并过 `task check`。

**Files:**
- Modify: `docs/superpowers/specs/2026-06-09-ccr-chain-design.md`（隔离段落）
- Modify: `CLAUDE.md`（chain v0.3 一行描述里「isolate worktree 隔离」→「可插拔隔离 + 成果交回」）

- [ ] **Step 1: 更新设计文档隔离语义**

在 `docs/superpowers/specs/2026-06-09-ccr-chain-design.md` 找到描述 isolate/worktree 的段落，替换为：

```markdown
- **隔离（isolate）**：可插拔 Isolator。git 目录用临时 worktree（每段 ccr 兜底提交，避免 agent 不提交导致丢失）；非 git 目录用 copydir 快照。结束三态：跑完且审查 pass→本地 merge 合回当前分支并删临时分支；needs-work / 报错 / 用户退出→保留成果并打印取回路径，**绝不静默销毁**。详见 specs/2026-06-11-ccr-chain-isolation-design.md。
```

- [ ] **Step 2: 更新 CLAUDE.md 一行**

在 `CLAUDE.md` 的 chain（v0.3）描述里，把结尾 `isolate worktree 隔离` 改为 `可插拔隔离（worktree/copydir）+ 三态成果交回`。

- [ ] **Step 3: 全套校验**

Run: `task check`
Expected: fmt/vet/lint/test 全绿。若 lint 报 `errcheck`（如未检查的 `gitIn` 返回），对确属 best-effort 的清理调用保持 `_, _ =` 赋值形式（已在代码中如此写）。

- [ ] **Step 4: 提交**

```bash
git add docs/superpowers/specs/2026-06-09-ccr-chain-design.md CLAUDE.md
git commit -m "docs: document pluggable chain isolation and result handback"
```

---

## Self-Review

**Spec coverage（对照 isolation-design.md）：**
- 可插拔 `Isolator` 接口（4 方法）→ Task 1。✓
- git 判定选 worktree / copydir → Task 1 `newIsolator`/`isInsideWorkTree`。✓
- worktree Setup/SealSegment（残留才补提交、排除 `.ccr-chain`）→ Task 2。✓
- worktree Integrate（ff→普通 merge→失败转 Abandon）+ 成功才删分支 → Task 3。✓
- worktree Abandon 保留分支、只删临时目录 → Task 3。✓
- copydir Setup/Seal(no-op)/Integrate(新增改动拷回、不镜像删除)/Abandon(保留副本) → Task 4。✓
- 结束三态（pass→Integrate / needs-work→Abandon / 报错·退出→Abandon）→ Task 5。✓
- 不隔离时不建 Isolator、就地跑、行为不变 → Task 5（`if c.Isolate` 包裹，`iso==nil` 各处短路）。✓
- 删旧无条件 cleanup（`CreateWorktree`/`worktree.go`）→ Task 1。✓
- 事故回归（agent 没提交也不丢）→ Task 5 `TestOrchestrate_isolate成功合回成果` + `_段报错保留成果`。✓
- 文档同步 → Task 6。✓
- **非目标**（容器隔离、显式丢弃旋钮、冲突自动解决、copydir exclude/删除镜像）均未实现，符合 YAGNI。✓

**Placeholder scan：** 唯一占位是 Task 1 的 `var _ = ...` 编译桥，Task 2/4 明确指示删除——非交付物，已标注。无 TBD/“适当处理”等。

**Type consistency：** `Isolator` 四方法签名在 Task 1 定义，Task 2–4 实现一致；`worktreeIsolator{repo,label,branch,dir}` 与 `copydirIsolator{src,dir}` 字段在 Task 1 声明、后续任务使用一致；orchestrate 调用的 `newIsolator/Setup/SealSegment/Integrate/Abandon` 与定义匹配；`o.Out` 字段 Task 5 定义并在测试中赋值。
