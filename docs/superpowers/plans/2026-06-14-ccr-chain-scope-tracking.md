# ccr chain 任务范围追踪（相关文件集传递）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 chain 在段间传递"本次链已改动的文件集"，注入后续段 prompt，把审查/修复范围收敛到真正相关的文件上。

**Architecture:** 新增 `ChangeTracker` 抽象（git 用 `git diff --name-only <baseSHA>`，非 git 用 size+mtime 快照），独立于 Isolator。orchestrator 在链开始记录基准、每段循环顶部累加链级总集、非空时把文件清单追加进 prompt。物理硬围栏（第 2 层）保留不动，本设计是叠加的第 3 层软提示。

**Tech Stack:** Go（CGO_ENABLED=0）、标准库 `os`/`path/filepath`/`os/exec`、现有 `gitIn`/`isInsideWorkTree` 工具。

参照设计：`docs/superpowers/specs/2026-06-14-ccr-chain-scope-tracking-design.md`

---

## 文件结构

- **新增 `internal/chain/changetracker.go`**：`ChangeTracker` 接口 + `gitTracker` + `fsTracker` + `newChangeTracker` + `RelevantFilesNote`。单一职责：追踪 workdir 改动文件集 + 生成 prompt 提示。
- **新增 `internal/chain/changetracker_test.go`**：上述单元测试。
- **改 `internal/chain/orchestrate.go`**：链开始 `Baseline()`、循环顶部累加并集、prompt 注入。
- **改 `internal/chain/orchestrate_test.go`**：3 段链注入测试 + 事故回归测试。
- **改 `internal/chain/templates/plan-impl-review.yaml`**：review 段手写的"只看当前工作目录"长段换成简短引导（清单由注入提供）。

> 约定：提交信息走 Conventional Commits；提交前可跑 `task check`，但单任务内用 `go test ./internal/chain/ -run <name> -v` 做快速 TDD 反馈。

---

### Task 1: ChangeTracker 接口 + newChangeTracker 选择器

**Files:**
- Create: `internal/chain/changetracker.go`
- Test: `internal/chain/changetracker_test.go`

- [ ] **Step 1: 写失败测试（选择器按 git 与否分流）**

写入 `internal/chain/changetracker_test.go`：

```go
package chain

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestNewChangeTracker_git目录选gitTracker(t *testing.T) {
	repo := initRepo(t)
	tr := newChangeTracker(repo)
	if _, ok := tr.(*gitTracker); !ok {
		t.Errorf("git 工作树应选 gitTracker, got %T", tr)
	}
}

func TestNewChangeTracker_非git目录选fsTracker(t *testing.T) {
	dir := t.TempDir()
	tr := newChangeTracker(dir)
	if _, ok := tr.(*fsTracker); !ok {
		t.Errorf("非 git 目录应选 fsTracker, got %T", tr)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/chain/ -run TestNewChangeTracker -v`
Expected: 编译失败，`undefined: newChangeTracker` / `gitTracker` / `fsTracker`

- [ ] **Step 3: 写接口与选择器骨架**

写入 `internal/chain/changetracker.go`（gitTracker/fsTracker 先放最小可编译骨架，后续任务填实现）：

```go
package chain

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ChangeTracker 追踪一条链从开始到当前在 workdir 内改动了哪些文件。
// 与 Isolator 的 git/非 git 判定同源，但独立：isolate=false 就地执行时没有
// Isolator，仍需追踪范围。
type ChangeTracker interface {
	// Baseline 在链开始时记录基准（git: HEAD + 已脏文件集；fs: size/mtime 快照）。
	Baseline() error
	// ChangedFiles 返回相对基准、被本链改动的文件（相对 workdir、已排序）。
	ChangedFiles() ([]string, error)
}

// newChangeTracker 按 workdir 是否在 git 工作树内选实现。
func newChangeTracker(workdir string) ChangeTracker {
	if isInsideWorkTree(workdir) {
		return &gitTracker{workdir: workdir}
	}
	return &fsTracker{workdir: workdir}
}

// gitTracker 用 git diff 追踪改动（worktree isolate 与就地 git 模式统一）。
type gitTracker struct {
	workdir   string
	baseSHA   string
	baseDirty map[string]bool
}

func (g *gitTracker) Baseline() error      { return nil }
func (g *gitTracker) ChangedFiles() ([]string, error) { return nil, nil }

// fsTracker 用 size+mtime 快照追踪改动（非 git 目录）。
type fsTracker struct {
	workdir  string
	snapshot map[string]fileSig
}

type fileSig struct {
	size  int64
	mtime int64
}

func (f *fsTracker) Baseline() error      { return nil }
func (f *fsTracker) ChangedFiles() ([]string, error) { return nil, nil }
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/chain/ -run TestNewChangeTracker -v`
Expected: PASS（两个选择器测试）

- [ ] **Step 5: 提交**

```bash
git add internal/chain/changetracker.go internal/chain/changetracker_test.go
git commit -m "feat: add ChangeTracker interface and git/non-git selector"
```

---

### Task 2: gitTracker 实现

**Files:**
- Modify: `internal/chain/changetracker.go`
- Test: `internal/chain/changetracker_test.go`

- [ ] **Step 1: 写失败测试**

追加到 `internal/chain/changetracker_test.go`。`writeFile` 是本任务引入的小助手：

```go
func writeFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGitTracker_捕获新增与修改(t *testing.T) {
	repo := initRepo(t) // 已含已提交的 f.txt
	g := &gitTracker{workdir: repo}
	if err := g.Baseline(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, "new.txt", "新文件")        // untracked
	writeFile(t, repo, "f.txt", "改了内容")          // tracked 修改
	got, err := g.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"f.txt", "new.txt"}
	if !equalStrs(got, want) {
		t.Errorf("ChangedFiles = %v, want %v", got, want)
	}
}

func TestGitTracker_扣除链前已脏文件(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "dirty.txt", "链开始前就脏") // baseline 前已脏
	g := &gitTracker{workdir: repo}
	if err := g.Baseline(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, "real.txt", "本链改动")
	got, err := g.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if !equalStrs(got, []string{"real.txt"}) {
		t.Errorf("应扣除链前已脏的 dirty.txt, got %v", got)
	}
}

func TestGitTracker_已提交改动仍计入(t *testing.T) {
	repo := initRepo(t)
	g := &gitTracker{workdir: repo}
	if err := g.Baseline(); err != nil {
		t.Fatal(err)
	}
	// 模拟 SealSegment：改文件后提交
	writeFile(t, repo, "sealed.txt", "封存的成果")
	if out, err := gitIn(repo, "add", "-A"); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	if out, err := gitIn(repo, "commit", "-m", "seal"); err != nil {
		t.Fatalf("git commit: %v %s", err, out)
	}
	got, err := g.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if !equalStrs(got, []string{"sealed.txt"}) {
		t.Errorf("diff baseSHA 应覆盖已提交改动, got %v", got)
	}
}

func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/chain/ -run TestGitTracker -v`
Expected: FAIL（`ChangedFiles` 返回 nil，断言不符）

- [ ] **Step 3: 实现 gitTracker**

替换 `changetracker.go` 里 gitTracker 的两个方法：

```go
func (g *gitTracker) Baseline() error {
	out, err := gitIn(g.workdir, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("读 baseSHA 失败: %v %s", err, out)
	}
	g.baseSHA = strings.TrimSpace(string(out))
	g.baseDirty = map[string]bool{}
	for _, f := range g.statusFiles() {
		g.baseDirty[f] = true
	}
	return nil
}

func (g *gitTracker) ChangedFiles() ([]string, error) {
	set := map[string]bool{}
	// tracked：工作树（含已提交封存）相对 baseSHA 的改动，不含 untracked。
	if out, err := gitIn(g.workdir, "diff", "--name-only", g.baseSHA); err == nil {
		for _, f := range splitLines(string(out)) {
			set[f] = true
		}
	}
	// untracked：新建但未 git add 的文件。
	if out, err := gitIn(g.workdir, "ls-files", "--others", "--exclude-standard"); err == nil {
		for _, f := range splitLines(string(out)) {
			set[f] = true
		}
	}
	var files []string
	for f := range set {
		if !g.baseDirty[f] { // 扣掉链开始前就脏的
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files, nil
}

// statusFiles 返回 git status --porcelain 列出的文件名（含 untracked）。
func (g *gitTracker) statusFiles() []string {
	out, err := gitIn(g.workdir, "status", "--porcelain")
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range splitLines(string(out)) {
		// porcelain 行形如 " M path" / "?? path" / "R  old -> new"；取末尾路径。
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		// 去掉前两位状态码 + 空格
		if len(line) > 3 {
			s = strings.TrimSpace(line[3:])
		}
		if idx := strings.Index(s, " -> "); idx >= 0 {
			s = s[idx+len(" -> "):]
		}
		files = append(files, s)
	}
	return files
}

// splitLines 按行切并丢空行（git --name-only 等输出尾随空行）。
func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return out
}
```

在文件顶部 import 补 `"fmt"`：

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/chain/ -run TestGitTracker -v`
Expected: PASS（三个 gitTracker 测试 + equalStrs 助手编译通过）

- [ ] **Step 5: 提交**

```bash
git add internal/chain/changetracker.go internal/chain/changetracker_test.go
git commit -m "feat: implement gitTracker using diff against baseline SHA"
```

---

### Task 3: fsTracker 实现

**Files:**
- Modify: `internal/chain/changetracker.go`
- Test: `internal/chain/changetracker_test.go`

- [ ] **Step 1: 写失败测试**

追加到 `changetracker_test.go`：

```go
func TestFsTracker_捕获新增与修改(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "exist.txt", "原始")
	f := &fsTracker{workdir: dir}
	if err := f.Baseline(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "new.txt", "新文件")
	// 改 exist.txt 内容并把 mtime 推后，确保签名变化（size 也变）。
	writeFile(t, dir, "exist.txt", "改了而且更长")
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(filepath.Join(dir, "exist.txt"), future, future)
	got, err := f.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if !equalStrs(got, []string{"exist.txt", "new.txt"}) {
		t.Errorf("ChangedFiles = %v, want [exist.txt new.txt]", got)
	}
}

func TestFsTracker_跳过git与ccrchain(t *testing.T) {
	dir := t.TempDir()
	f := &fsTracker{workdir: dir}
	if err := f.Baseline(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".git/config", "应忽略")
	writeFile(t, dir, ".ccr-chain/verdict", "pass")
	writeFile(t, dir, "real.txt", "应计入")
	got, err := f.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if !equalStrs(got, []string{"real.txt"}) {
		t.Errorf("应只含 real.txt, got %v", got)
	}
}
```

import 补 `"time"`（测试文件顶部）：

```go
import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/chain/ -run TestFsTracker -v`
Expected: FAIL（`ChangedFiles` 返回 nil）

- [ ] **Step 3: 实现 fsTracker**

替换 `changetracker.go` 里 fsTracker 的两个方法，并加一个共享的 walk 助手：

```go
func (f *fsTracker) Baseline() error {
	snap, err := f.scan()
	if err != nil {
		return err
	}
	f.snapshot = snap
	return nil
}

func (f *fsTracker) ChangedFiles() ([]string, error) {
	cur, err := f.scan()
	if err != nil {
		return nil, err
	}
	var files []string
	for rel, sig := range cur {
		old, ok := f.snapshot[rel]
		if !ok || old != sig { // 新增 或 size/mtime 变化
			files = append(files, rel)
		}
	}
	sort.Strings(files)
	return files, nil
}

// scan 遍历 workdir，返回 相对路径 -> 签名；跳过 .git / .ccr-chain。
func (f *fsTracker) scan() (map[string]fileSig, error) {
	out := map[string]fileSig{}
	err := filepath.WalkDir(f.workdir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(f.workdir, p)
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
		info, err := d.Info()
		if err != nil {
			return nil // 文件可能刚被删，忽略
		}
		out[filepath.ToSlash(rel)] = fileSig{size: info.Size(), mtime: info.ModTime().UnixNano()}
		return nil
	})
	return out, err
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/chain/ -run TestFsTracker -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/chain/changetracker.go internal/chain/changetracker_test.go
git commit -m "feat: implement fsTracker using size+mtime snapshot"
```

---

### Task 4: RelevantFilesNote prompt 提示生成

**Files:**
- Modify: `internal/chain/changetracker.go`
- Test: `internal/chain/changetracker_test.go`

- [ ] **Step 1: 写失败测试**

追加到 `changetracker_test.go`：

```go
func TestRelevantFilesNote_空集返回空串(t *testing.T) {
	if RelevantFilesNote(nil) != "" {
		t.Error("空集应返回空串")
	}
	if RelevantFilesNote([]string{}) != "" {
		t.Error("空 slice 应返回空串")
	}
}

func TestRelevantFilesNote_含全部文件名(t *testing.T) {
	note := RelevantFilesNote([]string{"a.go", "b/c.go"})
	if !strings.Contains(note, "a.go") || !strings.Contains(note, "b/c.go") {
		t.Errorf("提示应含全部文件名: %q", note)
	}
	if !strings.Contains(note, "聚焦") {
		t.Errorf("提示应含聚焦引导语: %q", note)
	}
}
```

测试文件 import 补 `"strings"`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/chain/ -run TestRelevantFilesNote -v`
Expected: FAIL（`undefined: RelevantFilesNote`）

- [ ] **Step 3: 实现 RelevantFilesNote**

追加到 `changetracker.go`：

```go
// RelevantFilesNote 生成追加到后续段 prompt 末尾的范围提示；空集返回空串。
// 这是软提示（不是硬围栏）：引导 agent 聚焦本链已改文件，物理边界由 guard 兜底。
func RelevantFilesNote(files []string) string {
	if len(files) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n---\n[本次链已改动以下文件，请聚焦这些文件，不要读取无关或外部路径]\n")
	for _, f := range files {
		b.WriteString("- ")
		b.WriteString(f)
		b.WriteString("\n")
	}
	return b.String()
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/chain/ -run TestRelevantFilesNote -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/chain/changetracker.go internal/chain/changetracker_test.go
git commit -m "feat: add RelevantFilesNote to render changed-file prompt hint"
```

---

### Task 5: Orchestrator 接入（Baseline + 累加 + prompt 注入）

**Files:**
- Modify: `internal/chain/orchestrate.go:47-121`
- Test: `internal/chain/orchestrate_test.go`

- [ ] **Step 1: 写失败测试**

追加到 `orchestrate_test.go`。注入的 fake runSegment 在 workdir 真写文件，模拟 agent 改动：

```go
func TestOrchestrate_注入相关文件集到后续段(t *testing.T) {
	dir := t.TempDir() // 非 git → fsTracker
	c := Chain{
		Workdir: dir,
		Segments: []Segment{
			{Name: "plan", Profile: "strong", Prompt: "规划"},
			{Name: "impl", Profile: "strong", Prompt: "实现"},
			{Name: "review", Profile: "cheap", Prompt: "审查"},
		},
	}
	var seenPrompts []string
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		seenPrompts = append(seenPrompts, spec.Prompt)
		// impl 段写两个文件，模拟 agent 改动
		if seg.Name == "impl" {
			_ = os.WriteFile(filepath.Join(dir, "foo.go"), []byte("x"), 0o644)
			_ = os.WriteFile(filepath.Join(dir, "bar.go"), []byte("y"), 0o644)
		}
		return seg.Name + "-out", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	// 首段 prompt 无清单（链开始无改动）
	if strings.Contains(seenPrompts[0], "本次链已改动") {
		t.Errorf("首段不应有文件清单: %q", seenPrompts[0])
	}
	// review 段（第 3 段）应看到 impl 写的两个文件
	if !strings.Contains(seenPrompts[2], "foo.go") || !strings.Contains(seenPrompts[2], "bar.go") {
		t.Errorf("review 段应含相关文件清单: %q", seenPrompts[2])
	}
}

func TestOrchestrate_tracker出错不打断链(t *testing.T) {
	// workdir 指向不存在的路径：tracker Baseline/ChangedFiles 出错，但链应照常跑完。
	c := Chain{
		Workdir: t.TempDir(),
		Segments: []Segment{
			{Name: "a", Profile: "strong", Prompt: "x"},
			{Name: "b", Profile: "strong", Prompt: "y"},
		},
	}
	o := NewOrchestrator(testReg())
	o.Auto = true
	ran := 0
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		ran++
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if ran != 2 {
		t.Errorf("两段都应跑完, ran=%d", ran)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/chain/ -run TestOrchestrate_注入相关文件集到后续段 -v`
Expected: FAIL（review 段 prompt 不含 foo.go，因为还没接入）

- [ ] **Step 3: 接入 orchestrate.go**

在 `orchestrate.go` 的 `Run` 里，`workdir` 定稿之后（即 `settingsRoot` 创建之后、`for` 循环之前）插入 tracker 初始化：

```go
	// 任务边界（第 3 层）：追踪本链改动文件集，注入后续段 prompt 作软提示。
	// best-effort——失败则后续不注入，绝不打断链（与 segmentDiffStat 同语义）。
	tracker := newChangeTracker(workdir)
	_ = tracker.Baseline()
	relevant := map[string]bool{} // 链级只增总集
```

然后在 `for` 循环顶部，`seg := c.Segments[i]` 之后、`renderedPrompt := Render(...)` 之前插入累加逻辑，并在 `renderedPrompt` 计算后追加提示。具体改动 `renderedPrompt` 这几行：

原代码：
```go
		renderedPrompt := Render(seg.Prompt, prev, o.Input)
		if seg.Review {
			renderedPrompt += ReviewInstruction()
		}
```

改为：
```go
		// 累加上一段（已 SealSegment）的改动到链级总集，并集防"建了又删"反复。
		if files, err := tracker.ChangedFiles(); err == nil {
			for _, f := range files {
				relevant[f] = true
			}
		}
		renderedPrompt := Render(seg.Prompt, prev, o.Input)
		if seg.Review {
			renderedPrompt += ReviewInstruction()
		}
		if note := RelevantFilesNote(sortedKeys(relevant)); note != "" {
			renderedPrompt += note
		}
```

在文件末尾（`abandon` 函数之后）加助手：

```go
// sortedKeys 返回 map 的键，已排序——给 RelevantFilesNote 稳定输出。
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
```

在 `orchestrate.go` 顶部 import 补 `"sort"`：

```go
import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ttsimon/cc-run/internal/registry"
	"github.com/ttsimon/cc-run/internal/ui"
)
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/chain/ -run TestOrchestrate -v`
Expected: PASS（新增两个测试 + 原有 orchestrate 测试不回归）

- [ ] **Step 5: 提交**

```bash
git add internal/chain/orchestrate.go internal/chain/orchestrate_test.go
git commit -m "feat: inject chain-relevant changed files into downstream segment prompts"
```

---

### Task 6: 事故回归测试（父级 git、temp/ 非 git）

**Files:**
- Test: `internal/chain/orchestrate_test.go`

- [ ] **Step 1: 写回归测试**

复现记忆里的事故布局：父目录是 git 仓库，子目录 `temp/` 非 git。验证 fsTracker 走子目录、相关文件集不含父仓库文件。

追加到 `orchestrate_test.go`：

```go
func TestOrchestrate_回归_非git子目录不含父仓库文件(t *testing.T) {
	repo := initRepo(t) // 父级是 git，含已提交 f.txt
	sub := filepath.Join(repo, "temp")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	c := Chain{
		Workdir: sub, // 非 git 子目录
		Segments: []Segment{
			{Name: "impl", Profile: "strong", Prompt: "实现"},
			{Name: "review", Profile: "cheap", Prompt: "审查"},
		},
	}
	var reviewPrompt string
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		if seg.Name == "impl" {
			_ = os.WriteFile(filepath.Join(sub, "result.txt"), []byte("成果"), 0o644)
		}
		if seg.Name == "review" {
			reviewPrompt = spec.Prompt
		}
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	// review 段应看到子目录里的 result.txt
	if !strings.Contains(reviewPrompt, "result.txt") {
		t.Errorf("review 应含子目录成果 result.txt: %q", reviewPrompt)
	}
	// 但绝不能含父仓库的 f.txt
	if strings.Contains(reviewPrompt, "f.txt") {
		t.Errorf("review 不应含父仓库文件 f.txt: %q", reviewPrompt)
	}
}
```

- [ ] **Step 2: 跑测试**

Run: `go test ./internal/chain/ -run TestOrchestrate_回归 -v`
Expected: PASS（newChangeTracker 对 `sub` 用 fsTracker——`sub` 不是 git 工作树根；fsTracker 只扫 `sub` 内，看不到父仓库 f.txt）

> 若此处意外 FAIL（reviewPrompt 含 f.txt），说明 `isInsideWorkTree(sub)` 误判为 true（git 从子目录向上找到了父 .git）。这正是 `gitIn` 里 `GIT_CEILING_DIRECTORIES` 要解决的——确认硬围栏 diff 已在工作树中保留。诊断方向：检查 `isInsideWorkTree` 是否走 `gitIn`、ceiling 是否生效。

- [ ] **Step 3: 提交**

```bash
git add internal/chain/orchestrate_test.go
git commit -m "test: regression for non-git subdir not leaking parent repo files"
```

---

### Task 7: 模板精简（review 段引导语换成依赖注入）

**Files:**
- Modify: `internal/chain/templates/plan-impl-review.yaml`

- [ ] **Step 1: 改模板**

当前 review 段（未提交 diff 里加的）手写了两行长引导。文件清单现在由注入提供，把它精简为一句简短引导。

把 review 段的 prompt 从：
```yaml
    prompt: |
      审查本次改动是否符合计划、有无明显 bug。
      只看当前工作目录（含子目录）的内容，不要读取外部路径或父级仓库的源码——
      你要审的是这条链刚刚产出的成果，不是 ccr 工具本身。
```
改为：
```yaml
    prompt: |
      审查本次改动是否符合计划、有无明显 bug。
      你要审的是这条链刚刚产出的成果（见下方文件清单），不是 ccr 工具本身。
```

- [ ] **Step 2: 验证模板能解析**

Run: `go test ./internal/chain/ -run TestParse -v`
Expected: PASS（模板 YAML 结构未变，解析不回归）

> 注：`plan-impl-review.yaml` 是模板，profile 名是占位（"改成另一家profile名"），不能直接跑。这里只验证 YAML 可解析；端到端手动验证见 Task 8。

- [ ] **Step 3: 提交**

```bash
git add internal/chain/templates/plan-impl-review.yaml
git commit -m "docs: slim review prompt now that changed-file list is injected"
```

---

### Task 8: 全量验证 + 手动端到端

**Files:** 无（验证任务）

- [ ] **Step 1: 跑 chain 包全量测试**

Run: `go test ./internal/chain/ -v`
Expected: 全部 PASS，无回归

- [ ] **Step 2: 跑提交前全套检查**

Run: `task check`
Expected: fmt + vet + lint + test 全绿。若 lint 报 `sortedKeys`/`splitLines` 等未用，回查接入点。

- [ ] **Step 3: 手动端到端（参照记忆里的常用测试布局）**

在 `temp/` 子目录放一份真实可跑的 `*.chain.yaml`（profile 填真实名如 DeepSeek，`isolate: false`），跑：

```bash
ccr chain temp/plan-impl-review.chain.yaml --input "加个小功能" -v
```

人工确认：
- review 段日志里 prompt 含 "[本次链已改动以下文件...]" 且列出 impl 段真实写的文件。
- 清单不含父仓库（ccr 自己）的文件。

> 这是手动验证，不进自动化测试（需真实 claude + profile）。

- [ ] **Step 4: 最终提交（若手动验证发现需微调）**

```bash
git add -A
git commit -m "test: verify chain scope tracking end-to-end"
```

---

## Self-Review

**1. Spec coverage：**
- ChangeTracker 接口 + 选择器 → Task 1 ✓
- gitTracker（diff baseSHA、扣 baseDirty、含 untracked、覆盖已提交）→ Task 2 ✓
- fsTracker（size+mtime 快照、跳过 .git/.ccr-chain）→ Task 3 ✓
- RelevantFilesNote（空集空串、非空含全部）→ Task 4 ✓
- Orchestrator 接入（Baseline、循环顶部累加并集、非空注入、首段不注入、best-effort 不打断）→ Task 5 ✓
- 事故回归（父 git、temp 非 git，不泄父仓库文件）→ Task 6 ✓
- 模板精简 → Task 7 ✓
- 物理硬围栏保留不动 → 全程未触碰 security.go/cli.go，spec 第 2 层地板保留 ✓
- 链级累加总集（决策 2）→ Task 5 用 `relevant` map 并集 ✓
- guard 不变、文件集不进 guard（决策 1）→ 未改 guard 相关代码 ✓

**2. Placeholder scan：** 无 TBD/TODO；每个 code step 含完整代码；测试含真实断言。✓

**3. Type consistency：**
- `ChangeTracker` 接口方法 `Baseline() error` / `ChangedFiles() ([]string, error)` 全任务一致 ✓
- `gitTracker{workdir, baseSHA, baseDirty}` / `fsTracker{workdir, snapshot}` / `fileSig{size, mtime}` 定义与使用一致 ✓
- `newChangeTracker(workdir string) ChangeTracker` 签名一致 ✓
- `RelevantFilesNote([]string) string` 在 Task 4 定义、Task 5 调用一致 ✓
- 助手 `sortedKeys`（orchestrate.go）、`splitLines`（changetracker.go）、`writeFile`/`equalStrs`（test）各定义一次 ✓
