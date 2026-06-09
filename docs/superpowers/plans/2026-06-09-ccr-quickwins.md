# ccr 快点小功能批（v0.2）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给 ccr 加一层旁挂元数据（别名 / 默认 / 上次用）和 shell 补全，让日常拉起常用 provider 更省事。

**Architecture:** 在 `~/.ccr/` 下旁挂 `overlay.json`（用户意图：别名表 + 默认）与 `state.json`（运行痕迹：上次用的），因为 cc-switch 库只读、写不回去。`internal/overlay` 负责读写；`internal/registry` 扩展按名解析（特殊记号 → 精确名 → 别名 → 模糊子串）；`internal/completion` 生成各 shell 补全脚本并提供一键安装；`internal/cli` 接上新子命令与解析、并在拉起时记录「上次」。

**Tech Stack:** Go（CGO_ENABLED=0），标准库 `encoding/json`/`os`/`path/filepath`，TUI 复用现有 `charmbracelet/huh`（`tui.SelectProfile` 已能接受任意 profile 子集，无需改动）。测试沿用项目约定：纯逻辑全单测、用 `userHomeDir` 变量替换 home、中文测试名。

> **规约提醒**：提交信息走 Conventional Commits（commit-msg 钩子强制），只有 `feat:`/`fix:` 进发布说明。每个 Task 末尾 commit。master 受保护走 PR-only，本计划在分支上实现。提交前可跑 `task check`。

---

## File Structure

- `internal/overlay/overlay.go`（新建）—— `Overlay`/`State` 类型 + `LoadOverlay`/`SaveOverlay`/`LoadState`/`SaveState`。单一职责：旁挂元数据的读写。
- `internal/overlay/overlay_test.go`（新建）—— 读写往返、缺文件回退、坏文件回退。
- `internal/registry/registry.go`（改）—— 抽出 `exactMatches` helper；新增 `LookupResult` 类型与 `Lookup(query, aliases)` 方法（特殊记号除外的解析链）。
- `internal/registry/registry_test.go`（改）—— 补别名命中、模糊唯一命中、模糊多命中、零命中的用例。
- `internal/completion/completion.go`（新建）—— `Script(shell)` 静态脚本 + `Install`/`Uninstall`（幂等增删 rc 文件块）+ `DetectShell` + 标记常量。
- `internal/completion/completion_test.go`（新建）—— 脚本含关键标记、安装幂等、卸载干净。
- `internal/cli/cli.go`（改）—— 新子命令 `alias`/`unalias`/`default`/`completion`/隐藏 `__complete_names`；arg 路径加特殊记号 `-`/`.`、走 `Lookup`、多命中转 TUI、拉起前记录「上次」；更新 `printUsage`。
- `internal/cli/cli_test.go`（改）—— 子命令解析与 overlay 落盘断言。
- `README.md`（改）—— 文档化新命令与补全安装。

> 落点对照现有 `internal/` 三层布局，新增两个聚焦小包（overlay、completion），不动 source/launcher/tui 的内部实现。

---

## Task 1: overlay 包 —— 旁挂元数据读写

**Files:**
- Create: `internal/overlay/overlay.go`
- Test: `internal/overlay/overlay_test.go`

- [ ] **Step 1: 写失败测试**

`internal/overlay/overlay_test.go`:

```go
package overlay

import (
	"os"
	"path/filepath"
	"testing"
)

func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = os.UserHomeDir })
	return home
}

func TestOverlay_往返(t *testing.T) {
	withHome(t)
	in := Overlay{Aliases: map[string]string{"prod": "cc-switch:DeepSeek"}, Default: "my-local"}
	if err := SaveOverlay(in); err != nil {
		t.Fatal(err)
	}
	out := LoadOverlay()
	if out.Default != "my-local" || out.Aliases["prod"] != "cc-switch:DeepSeek" {
		t.Errorf("往返不一致: %+v", out)
	}
}

func TestOverlay_缺文件回退空且别名可写(t *testing.T) {
	withHome(t)
	out := LoadOverlay()
	if out.Default != "" {
		t.Errorf("缺文件应空默认")
	}
	out.Aliases["x"] = "y" // 不应 panic（map 已初始化）
}

func TestOverlay_坏文件回退空(t *testing.T) {
	home := withHome(t)
	if err := os.MkdirAll(filepath.Join(home, ".ccr"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ccr", "overlay.json"), []byte("{bad json"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := LoadOverlay()
	if out.Default != "" || len(out.Aliases) != 0 {
		t.Errorf("坏文件应回退空")
	}
}

func TestState_往返(t *testing.T) {
	withHome(t)
	if err := SaveState(State{Last: "cc-switch:火山"}); err != nil {
		t.Fatal(err)
	}
	if LoadState().Last != "cc-switch:火山" {
		t.Errorf("state 往返失败")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/overlay/`
Expected: FAIL（`undefined: Overlay` 等，包还不存在）

- [ ] **Step 3: 写实现**

`internal/overlay/overlay.go`:

```go
// Package overlay 读写 ccr 旁挂在 ~/.ccr 的元数据：
// overlay.json 是用户意图（别名表 + 默认 profile），state.json 是运行痕迹（上次用的）。
// cc-switch 库只读，这些写不回去，故另存一份。
package overlay

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// userHomeDir 便于测试替换。
var userHomeDir = os.UserHomeDir

// Overlay 是用户显式设的元数据。
type Overlay struct {
	Aliases map[string]string `json:"aliases"` // 别名 -> profile 查询名（可为 source:name）
	Default string            `json:"default"` // 默认 profile 查询名
}

// State 是运行时自动记录的状态。
type State struct {
	Last string `json:"last"` // 上次成功拉起的 profile（source:name）
}

func ccrDir() string {
	home, err := userHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".ccr")
}

func overlayPath() string { return filepath.Join(ccrDir(), "overlay.json") }
func statePath() string   { return filepath.Join(ccrDir(), "state.json") }

// LoadOverlay 读 overlay.json；缺文件或坏文件回退到空 Overlay（Aliases 始终非 nil）。
func LoadOverlay() Overlay {
	o := Overlay{Aliases: map[string]string{}}
	if raw, err := os.ReadFile(overlayPath()); err == nil {
		_ = json.Unmarshal(raw, &o) // 坏文件忽略
	}
	if o.Aliases == nil {
		o.Aliases = map[string]string{}
	}
	return o
}

// SaveOverlay 写 overlay.json（0600，目录 0700）。
func SaveOverlay(o Overlay) error {
	return writeJSON(overlayPath(), o)
}

// LoadState 读 state.json；缺/坏回退空 State。
func LoadState() State {
	var s State
	if raw, err := os.ReadFile(statePath()); err == nil {
		_ = json.Unmarshal(raw, &s)
	}
	return s
}

// SaveState 写 state.json。
func SaveState(s State) error {
	return writeJSON(statePath(), s)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/overlay/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/overlay/
git commit -m "feat: add overlay package for aliases/default/last metadata"
```

---

## Task 2: registry 抽出 exactMatches helper（无行为变化的重构）

**Files:**
- Modify: `internal/registry/registry.go`

- [ ] **Step 1: 重构 Resolve 复用新 helper**

把 `Resolve` 里内联的匹配抽成私有 `exactMatches`，`Resolve` 改为调用它。现有 `registry_test.go` 已覆盖 Resolve 的三种结果，作为回归网。

替换 `internal/registry/registry.go` 中的 `Resolve` 函数（第 36–65 行）为：

```go
// Resolve 按 query 找唯一 Profile。query 可为 "name" 或 "source:name"。
func (r *Registry) Resolve(query string) (profile.Profile, error) {
	matches := r.exactMatches(query)
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		_, name := splitQuery(query)
		return profile.Profile{}, r.notFoundErr(name)
	default:
		var qualified []string
		for _, p := range matches {
			qualified = append(qualified, fmt.Sprintf("%s:%s", p.Source, p.Name))
		}
		return profile.Profile{}, fmt.Errorf(
			"名字 %q 有多个来源，请用限定名指定其一：%s",
			query, strings.Join(qualified, " 、 "),
		)
	}
}

// exactMatches 返回精确（含 source:name 限定）命中的 Profile 列表（0/1/多）。
func (r *Registry) exactMatches(query string) []profile.Profile {
	wantSource, name := splitQuery(query)
	var matches []profile.Profile
	for _, p := range r.profiles {
		if p.Name != name {
			continue
		}
		if wantSource != "" && string(p.Source) != wantSource {
			continue
		}
		matches = append(matches, p)
	}
	return matches
}
```

- [ ] **Step 2: 跑测试确认仍通过（回归）**

Run: `go test ./internal/registry/`
Expected: PASS（已有 4 个 Resolve/List 用例全绿）

- [ ] **Step 3: 提交**

```bash
git add internal/registry/registry.go
git commit -m "refactor: extract exactMatches helper in registry"
```

---

## Task 3: registry.Lookup —— 别名 + 模糊解析

**Files:**
- Modify: `internal/registry/registry.go`
- Test: `internal/registry/registry_test.go`

- [ ] **Step 1: 写失败测试**

在 `internal/registry/registry_test.go` 末尾追加：

```go
func TestLookup_精确命中(t *testing.T) {
	r := New(sample())
	res, err := r.Lookup("火山", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Profile.Name != "火山" || len(res.Candidates) != 0 {
		t.Errorf("应精确命中且无候选: %+v", res)
	}
}

func TestLookup_别名命中(t *testing.T) {
	r := New(sample())
	res, err := r.Lookup("vol", map[string]string{"vol": "火山"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Profile.Name != "火山" {
		t.Errorf("别名应解析到 火山: %+v", res)
	}
}

func TestLookup_模糊唯一命中(t *testing.T) {
	r := New(sample())
	res, err := r.Lookup("local", nil) // 仅 my-local 含 "local"
	if err != nil {
		t.Fatal(err)
	}
	if res.Profile.Name != "my-local" || len(res.Candidates) != 0 {
		t.Errorf("应模糊唯一命中 my-local: %+v", res)
	}
}

func TestLookup_模糊多命中返回候选(t *testing.T) {
	r := New(sample())
	res, err := r.Lookup("deep", nil) // 两个 DeepSeek 都含 "deep"
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 2 {
		t.Errorf("应返回 2 个候选交给 TUI: %+v", res)
	}
}

func TestLookup_零命中报错(t *testing.T) {
	r := New(sample())
	_, err := r.Lookup("zzz", nil)
	if err == nil {
		t.Fatal("零命中应报错")
	}
}

func TestLookup_精确优先于模糊(t *testing.T) {
	// 构造：精确名 "deep" 存在，同时它是别的名字的子串场景下，精确应直接命中。
	r := New([]profile.Profile{
		{Name: "deep", Source: profile.SourceCustom},
		{Name: "deepsea", Source: profile.SourceCustom},
	})
	res, err := r.Lookup("deep", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Profile.Name != "deep" || len(res.Candidates) != 0 {
		t.Errorf("精确名应优先于模糊: %+v", res)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/registry/ -run TestLookup`
Expected: FAIL（`r.Lookup undefined`）

- [ ] **Step 3: 写实现**

在 `internal/registry/registry.go` 末尾追加（`strings` 已 import）：

```go
// LookupResult 表示一次按名解析的结果：
// 命中唯一 → Profile 有值；模糊多命中 → Candidates 有值，交给 TUI 选择。
type LookupResult struct {
	Profile    profile.Profile
	Candidates []profile.Profile
}

// Lookup 按 query 解析，依次尝试：精确名/限定名 → 别名（精确）→ 模糊子串。
// aliases: 别名 -> 查询名，可为 nil。
func (r *Registry) Lookup(query string, aliases map[string]string) (LookupResult, error) {
	// 1) 精确名 / source:name
	switch m := r.exactMatches(query); len(m) {
	case 1:
		return LookupResult{Profile: m[0]}, nil
	case 0:
		// 落到别名 / 模糊
	default:
		var qualified []string
		for _, p := range m {
			qualified = append(qualified, fmt.Sprintf("%s:%s", p.Source, p.Name))
		}
		return LookupResult{}, fmt.Errorf(
			"名字 %q 有多个来源，请用限定名指定其一：%s",
			query, strings.Join(qualified, " 、 "),
		)
	}

	// 2) 别名（精确匹配别名键，目标再做精确解析）
	if target, ok := aliases[query]; ok {
		if m := r.exactMatches(target); len(m) == 1 {
			return LookupResult{Profile: m[0]}, nil
		}
		return LookupResult{}, fmt.Errorf("别名 %q 指向 %q，但它无法唯一解析", query, target)
	}

	// 3) 模糊：名字含 query（不分大小写）
	lower := strings.ToLower(query)
	var cand []profile.Profile
	for _, p := range r.profiles {
		if strings.Contains(strings.ToLower(p.Name), lower) {
			cand = append(cand, p)
		}
	}
	switch len(cand) {
	case 1:
		return LookupResult{Profile: cand[0]}, nil
	case 0:
		return LookupResult{}, r.notFoundErr(query)
	default:
		return LookupResult{Candidates: cand}, nil
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/registry/`
Expected: PASS（含新增 6 个 Lookup 用例 + 原有用例）

- [ ] **Step 5: 提交**

```bash
git add internal/registry/registry.go internal/registry/registry_test.go
git commit -m "feat: add registry Lookup with alias and fuzzy resolution"
```

---

## Task 4: completion 包 —— 脚本生成

**Files:**
- Create: `internal/completion/completion.go`
- Test: `internal/completion/completion_test.go`

> 补全脚本是静态的，但 profile 名字是动态的：脚本运行时调用隐藏命令 `ccr __complete_names` 取当前名字列表（Task 6 实现该命令）。

- [ ] **Step 1: 写失败测试**

`internal/completion/completion_test.go`:

```go
package completion

import (
	"strings"
	"testing"
)

func TestScript_各shell含关键标记(t *testing.T) {
	cases := map[string]string{
		"bash":       "complete -F _ccr ccr",
		"zsh":        "#compdef ccr",
		"powershell": "Register-ArgumentCompleter",
	}
	for shell, marker := range cases {
		s, err := Script(shell)
		if err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		if !strings.Contains(s, marker) {
			t.Errorf("%s 脚本缺标记 %q", shell, marker)
		}
		if !strings.Contains(s, "__complete_names") {
			t.Errorf("%s 脚本应调用 __complete_names 取动态名字", shell)
		}
	}
}

func TestScript_未知shell报错(t *testing.T) {
	if _, err := Script("fish"); err == nil {
		t.Fatal("未支持的 shell 应报错")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/completion/`
Expected: FAIL（包不存在）

- [ ] **Step 3: 写实现**

`internal/completion/completion.go`:

```go
// Package completion 生成各 shell 的补全脚本，并提供一键安装/卸载到 shell 配置文件。
package completion

import "fmt"

// Script 返回指定 shell 的补全脚本。支持 bash / zsh / powershell。
// 脚本运行时调用 `ccr __complete_names` 取动态的 profile 名/别名。
func Script(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashScript, nil
	case "zsh":
		return zshScript, nil
	case "powershell", "pwsh":
		return pwshScript, nil
	default:
		return "", fmt.Errorf("不支持的 shell：%q（支持 bash/zsh/powershell）", shell)
	}
}

const bashScript = `# ccr bash 补全
_ccr() {
    local cur cmds names
    cur="${COMP_WORDS[COMP_CWORD]}"
    if [ "$COMP_CWORD" -eq 1 ]; then
        cmds="ls show edit alias unalias default completion -h --help -v --version"
        names="$(ccr __complete_names 2>/dev/null)"
        COMPREPLY=( $(compgen -W "${cmds} ${names}" -- "${cur}") )
    fi
}
complete -F _ccr ccr
`

const zshScript = `#compdef ccr
_ccr() {
    local -a cmds names
    cmds=(ls show edit alias unalias default completion)
    names=(${(f)"$(ccr __complete_names 2>/dev/null)"})
    _describe 'command' cmds
    _describe 'profile' names
}
compdef _ccr ccr
`

const pwshScript = `# ccr PowerShell 补全
Register-ArgumentCompleter -Native -CommandName ccr -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $cmds = @('ls','show','edit','alias','unalias','default','completion')
    $names = @(ccr __complete_names 2>$null)
    @($cmds + $names) | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/completion/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/completion/
git commit -m "feat: generate shell completion scripts for bash/zsh/powershell"
```

---

## Task 5: completion 一键安装 / 卸载（幂等）

**Files:**
- Modify: `internal/completion/completion.go`
- Modify: `internal/completion/completion_test.go`

> 安装 = 往 shell 的 rc 文件追加一段「带标记的引导块」，块内 `source <(ccr completion <shell>)`（pwsh 用 `Invoke-Expression`）。幂等：已有标记则跳过。卸载：删掉标记之间的块。rc 文件路径相对 home，便于测试用 TempDir 注入。

- [ ] **Step 1: 写失败测试**

在 `internal/completion/completion_test.go` 追加：

```go
import (
	"os"
	"path/filepath"
	// 注意：与上方已有 import 合并，勿重复
)

func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = os.UserHomeDir })
	return home
}

func TestInstall_写入并幂等(t *testing.T) {
	home := withHome(t)
	changed, path, err := Install("bash")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("首次安装应有改动")
	}
	if filepath.Dir(path) != home {
		t.Errorf("bash rc 应在 home 下: %q", path)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), markerStart) || !strings.Contains(string(raw), "ccr completion bash") {
		t.Errorf("rc 应含引导块: %s", raw)
	}
	// 第二次安装应幂等（无改动、不重复写）
	changed2, _, err := Install("bash")
	if err != nil {
		t.Fatal(err)
	}
	if changed2 {
		t.Error("重复安装应幂等无改动")
	}
	raw2, _ := os.ReadFile(path)
	if strings.Count(string(raw2), markerStart) != 1 {
		t.Errorf("引导块不应重复: %s", raw2)
	}
}

func TestUninstall_删干净(t *testing.T) {
	withHome(t)
	if _, _, err := Install("zsh"); err != nil {
		t.Fatal(err)
	}
	changed, path, err := Uninstall("zsh")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("卸载应有改动")
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), markerStart) {
		t.Errorf("卸载后不应残留标记: %s", raw)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/completion/ -run 'TestInstall|TestUninstall'`
Expected: FAIL（`Install`/`markerStart` 未定义）

- [ ] **Step 3: 写实现**

在 `internal/completion/completion.go` 顶部 import 改为：

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// userHomeDir 便于测试替换。
var userHomeDir = os.UserHomeDir

const (
	markerStart = "# >>> ccr completion >>>"
	markerEnd   = "# <<< ccr completion <<<"
)
```

在文件末尾追加：

```go
// rcPath 返回某 shell 的配置文件路径（相对 home）。
func rcPath(shell string) (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	switch shell {
	case "bash":
		return filepath.Join(home, ".bashrc"), nil
	case "zsh":
		return filepath.Join(home, ".zshrc"), nil
	case "powershell", "pwsh":
		// 跨平台简化：统一放 Documents/PowerShell/profile.ps1。
		return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), nil
	default:
		return "", fmt.Errorf("不支持的 shell：%q", shell)
	}
}

// loadLine 返回引导块中间那行（source / Invoke-Expression）。
func loadLine(shell string) string {
	switch shell {
	case "powershell", "pwsh":
		return "ccr completion powershell | Out-String | Invoke-Expression"
	default:
		return fmt.Sprintf("source <(ccr completion %s)", shell)
	}
}

// block 返回带标记的完整引导块。
func block(shell string) string {
	return fmt.Sprintf("%s\n%s\n%s\n", markerStart, loadLine(shell), markerEnd)
}

// Install 把引导块幂等地追加到 shell rc 文件。返回是否改动、rc 路径。
func Install(shell string) (changed bool, path string, err error) {
	path, err = rcPath(shell)
	if err != nil {
		return false, "", err
	}
	existing, _ := os.ReadFile(path) // 缺文件视为空
	if strings.Contains(string(existing), markerStart) {
		return false, path, nil // 已装，幂等
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, path, err
	}
	out := string(existing)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	out += block(shell)
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return false, path, err
	}
	return true, path, nil
}

// Uninstall 删掉 rc 文件中的引导块。返回是否改动、rc 路径。
func Uninstall(shell string) (changed bool, path string, err error) {
	path, err = rcPath(shell)
	if err != nil {
		return false, "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, path, nil // 没文件没块
	}
	s := string(raw)
	start := strings.Index(s, markerStart)
	end := strings.Index(s, markerEnd)
	if start < 0 || end < 0 || end < start {
		return false, path, nil
	}
	end += len(markerEnd)
	if end < len(s) && s[end] == '\n' {
		end++
	}
	out := s[:start] + s[end:]
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return false, path, err
	}
	return true, path, nil
}

// DetectShell 从环境猜当前 shell：$SHELL 的 basename，Windows 回退 powershell。
func DetectShell() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		base := filepath.Base(sh)
		switch {
		case strings.Contains(base, "zsh"):
			return "zsh"
		case strings.Contains(base, "bash"):
			return "bash"
		}
	}
	if os.Getenv("PSModulePath") != "" {
		return "powershell"
	}
	return ""
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/completion/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/completion/
git commit -m "feat: idempotent install/uninstall of completion into shell rc"
```

---

## Task 6: cli —— 隐藏 `__complete_names` 命令

**Files:**
- Modify: `internal/cli/cli.go`

> 补全脚本调用它取动态名字。输出每行一个：所有 profile 名 + 所有别名键。失败要安静（脚本里吞了 stderr），无配置时打印空、退出 0。

- [ ] **Step 1: 在 Execute 的 switch 里加分发**

在 `internal/cli/cli.go` 的子命令 `switch args[0]` 中，`case "edit":` 之后加：

```go
		case "__complete_names":
			return cmdCompleteNames(cfg, os.Stdout)
```

- [ ] **Step 2: 实现命令**

在 `internal/cli/cli.go` 末尾追加（`overlay` 包需在 import 块加 `"github.com/ttsimon/cc-run/internal/overlay"`）:

```go
// cmdCompleteNames 打印补全用的名字：profile 名 + 别名键，每行一个。
// 供补全脚本调用；任何缺失都安静处理，始终退出 0。
func cmdCompleteNames(cfg config.Config, out io.Writer) int {
	profiles, _ := source.LoadAll(
		source.NewCCSwitch(cfg.DB),
		source.NewCustomDir(cfg.ProfilesDir),
	)
	for _, p := range profiles {
		fmt.Fprintln(out, p.Name)
	}
	for alias := range overlay.LoadOverlay().Aliases {
		fmt.Fprintln(out, alias)
	}
	return 0
}
```

- [ ] **Step 3: 构建确认编译通过**

Run: `go build ./cmd/ccr`
Expected: 无错误

- [ ] **Step 4: 手动冒烟**

Run: `go run ./cmd/ccr __complete_names`
Expected: 打印你本机的 profile 名（每行一个）；无配置时无输出且退出 0。

- [ ] **Step 5: 提交**

```bash
git add internal/cli/cli.go
git commit -m "feat: add hidden __complete_names command for completion scripts"
```

---

## Task 7: cli —— `completion` 子命令

**Files:**
- Modify: `internal/cli/cli.go`

> `ccr completion <shell>` 打印脚本；`ccr completion install [shell]` 一键装（不带 shell 则自动探测）；`ccr completion install --uninstall [shell]` 卸载。

- [ ] **Step 1: 在 switch 里加分发**

在 `case "__complete_names":` 之前加：

```go
		case "completion":
			return runCompletion(args[1:], os.Stdout)
```

- [ ] **Step 2: 实现**

在 `internal/cli/cli.go` 末尾追加（import 块加 `"github.com/ttsimon/cc-run/internal/completion"`）:

```go
// runCompletion 处理 `ccr completion ...`。
func runCompletion(args []string, out io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "用法: ccr completion <bash|zsh|powershell> | ccr completion install [shell] [--uninstall]")
		return 1
	}

	if args[0] == "install" {
		uninstall := false
		shell := ""
		for _, a := range args[1:] {
			if a == "--uninstall" {
				uninstall = true
			} else {
				shell = a
			}
		}
		if shell == "" {
			shell = completion.DetectShell()
		}
		if shell == "" {
			fmt.Fprintln(os.Stderr, "无法探测当前 shell，请显式指定：ccr completion install <bash|zsh|powershell>")
			return 1
		}
		if uninstall {
			changed, path, err := completion.Uninstall(shell)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			if changed {
				fmt.Fprintf(out, "已从 %s 移除补全。重开终端生效。\n", path)
			} else {
				fmt.Fprintf(out, "%s 中未发现补全，无需移除。\n", path)
			}
			return 0
		}
		changed, path, err := completion.Install(shell)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if changed {
			fmt.Fprintf(out, "已把补全写入 %s。重开终端或 source 它生效。\n", path)
		} else {
			fmt.Fprintf(out, "%s 已包含补全，无需重复。\n", path)
		}
		return 0
	}

	// 否则 args[0] 当作 shell，打印脚本。
	script, err := completion.Script(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprint(out, script)
	return 0
}
```

- [ ] **Step 3: 构建 + 手动冒烟**

Run: `go run ./cmd/ccr completion bash`
Expected: 打印 bash 补全脚本（含 `complete -F _ccr ccr`）。

Run: `go run ./cmd/ccr completion install --uninstall bash`
Expected: 提示「未发现补全，无需移除」（因为还没装）。

- [ ] **Step 4: 提交**

```bash
git add internal/cli/cli.go
git commit -m "feat: add completion subcommand (print/install/uninstall)"
```

---

## Task 8: cli —— `alias` / `unalias` / `default` 子命令

**Files:**
- Modify: `internal/cli/cli.go`
- Test: `internal/cli/cli_test.go`

> 写 overlay 前校验目标能精确解析（用 `r.Resolve`），避免设出坏别名/坏默认。

- [ ] **Step 1: 写失败测试**

先看 `internal/cli/cli_test.go` 现有风格（已存在）。在其末尾追加（若文件已 import `os`/`testing`/`strings` 则勿重复）：

```go
func TestRunAlias_设置并落盘(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)            // unix
	t.Setenv("USERPROFILE", home)     // windows
	// 用一个能解析的目标：放一个自定义 profile。
	profDir := filepath.Join(home, ".ccr", "profiles")
	if err := os.MkdirAll(profDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profDir, "my-local.json"),
		[]byte(`{"env":{"ANTHROPIC_BASE_URL":"http://x"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CCR_DB", filepath.Join(home, "none.db")) // 不存在 → 仅自定义来源
	t.Setenv("CCR_PROFILES_DIR", profDir)

	if code := Execute([]string{"alias", "ml", "my-local"}); code != 0 {
		t.Fatalf("alias 应成功, code=%d", code)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".ccr", "overlay.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"ml"`) || !strings.Contains(string(raw), `my-local`) {
		t.Errorf("overlay 应含别名: %s", raw)
	}
}

func TestRunAlias_坏目标报错(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CCR_DB", filepath.Join(home, "none.db"))
	t.Setenv("CCR_PROFILES_DIR", filepath.Join(home, ".ccr", "profiles"))
	// 没有任何 profile → 目标无法解析（buildRegistry 也会报无配置）
	if code := Execute([]string{"alias", "x", "does-not-exist"}); code == 0 {
		t.Error("坏目标应非 0 退出")
	}
}
```

> 说明：`config.Load` 用 `os.UserHomeDir`（读 `HOME`/`USERPROFILE`），`overlay` 同理，故测试用 `t.Setenv` 指 home 即可。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/cli/ -run TestRunAlias`
Expected: FAIL（无 `alias` 分发，落到解析路径）

- [ ] **Step 3: 在 switch 里加分发**

在 `case "edit":` 之后加：

```go
		case "alias":
			return runAlias(cfg, args[1:], os.Stdout)
		case "unalias":
			return runUnalias(args[1:], os.Stdout)
		case "default":
			return runDefault(cfg, args[1:], os.Stdout)
```

- [ ] **Step 4: 实现三个命令**

在 `internal/cli/cli.go` 末尾追加（import 需有 `sort` —— 已存在）:

```go
// runAlias: 无参列出；`<别名> <目标>` 设置（校验目标可解析）。
func runAlias(cfg config.Config, args []string, out io.Writer) int {
	ov := overlay.LoadOverlay()
	if len(args) == 0 {
		keys := make([]string, 0, len(ov.Aliases))
		for k := range ov.Aliases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(out, "%-16s -> %s\n", k, ov.Aliases[k])
		}
		return 0
	}
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: ccr alias <别名> <目标profile>")
		return 1
	}
	alias, target := args[0], args[1]
	r, code := buildRegistry(cfg)
	if code != 0 {
		return code
	}
	if _, err := r.Resolve(target); err != nil {
		fmt.Fprintln(os.Stderr, "别名目标无法解析:", err)
		return 1
	}
	ov.Aliases[alias] = target
	if err := overlay.SaveOverlay(ov); err != nil {
		fmt.Fprintln(os.Stderr, "保存失败:", err)
		return 1
	}
	fmt.Fprintf(out, "已设别名: %s -> %s\n", alias, target)
	return 0
}

// runUnalias: 删一个别名。
func runUnalias(args []string, out io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "用法: ccr unalias <别名>")
		return 1
	}
	ov := overlay.LoadOverlay()
	if _, ok := ov.Aliases[args[0]]; !ok {
		fmt.Fprintf(os.Stderr, "没有别名 %q\n", args[0])
		return 1
	}
	delete(ov.Aliases, args[0])
	if err := overlay.SaveOverlay(ov); err != nil {
		fmt.Fprintln(os.Stderr, "保存失败:", err)
		return 1
	}
	fmt.Fprintf(out, "已删别名: %s\n", args[0])
	return 0
}

// runDefault: 无参打印当前默认；`<目标>` 设置（校验可解析）。
func runDefault(cfg config.Config, args []string, out io.Writer) int {
	ov := overlay.LoadOverlay()
	if len(args) == 0 {
		if ov.Default == "" {
			fmt.Fprintln(out, "（未设默认）用 `ccr default <名字>` 设置，之后 `ccr .` 直启。")
		} else {
			fmt.Fprintln(out, ov.Default)
		}
		return 0
	}
	target := args[0]
	r, code := buildRegistry(cfg)
	if code != 0 {
		return code
	}
	if _, err := r.Resolve(target); err != nil {
		fmt.Fprintln(os.Stderr, "默认目标无法解析:", err)
		return 1
	}
	ov.Default = target
	if err := overlay.SaveOverlay(ov); err != nil {
		fmt.Fprintln(os.Stderr, "保存失败:", err)
		return 1
	}
	fmt.Fprintf(out, "已设默认: %s（`ccr .` 直启）\n", target)
	return 0
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/cli/`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/cli/cli.go internal/cli/cli_test.go
git commit -m "feat: add alias/unalias/default subcommands backed by overlay"
```

---

## Task 9: cli —— 解析路径接特殊记号、模糊 TUI、记录「上次」

**Files:**
- Modify: `internal/cli/cli.go`

> 把现有「`ccr <name>`」分支换成：特殊记号 `-`/`.` → 翻译；`Lookup`（带别名）；多命中弹过滤 TUI；拉起前记录「上次」。无参 TUI 分支也记录「上次」。

- [ ] **Step 1: 替换解析与启动段**

把 `Execute` 中从 `// 其余：ccr <name> ...` 注释到函数 `return code2` 那段（当前第 55–85 行）整体替换为：

```go
	// 其余：ccr <name> [claude 参数...]、ccr -（上次）、ccr .（默认）、或 ccr（交互）。
	r, code := buildRegistry(cfg)
	if code != 0 {
		return code
	}

	var chosen profile.Profile
	var extra []string

	if len(args) == 0 {
		p, err := tui.SelectProfile(r.List())
		if err != nil {
			fmt.Fprintln(os.Stderr, "已取消")
			return 1
		}
		chosen = p
	} else {
		ov := overlay.LoadOverlay()
		query := args[0]
		extra = args[1:]

		// 特殊记号翻译。
		switch query {
		case "-":
			last := overlay.LoadState().Last
			if last == "" {
				fmt.Fprintln(os.Stderr, "还没有「上次」记录；先用 `ccr <名字>` 跑一次。")
				return 1
			}
			query = last
		case ".":
			if ov.Default == "" {
				fmt.Fprintln(os.Stderr, "还没设默认；用 `ccr default <名字>` 设置。")
				return 1
			}
			query = ov.Default
		}

		res, err := r.Lookup(query, ov.Aliases)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if len(res.Candidates) > 0 {
			p, err := tui.SelectProfile(res.Candidates)
			if err != nil {
				fmt.Fprintln(os.Stderr, "已取消")
				return 1
			}
			chosen = p
		} else {
			chosen = res.Profile
		}
	}

	// 记录「上次」（限定名，便于 `ccr -` 重放）。失败不致命。
	_ = overlay.SaveState(overlay.State{Last: fmt.Sprintf("%s:%s", chosen.Source, chosen.Name)})

	code2, err := launcher.New().Run(chosen, extra)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return code2
```

- [ ] **Step 2: 构建确认编译通过**

Run: `go build ./cmd/ccr`
Expected: 无错误

- [ ] **Step 3: 跑全量测试**

Run: `go test ./...`
Expected: PASS（所有包）

- [ ] **Step 4: 手动冒烟（需本机有 profile）**

- `go run ./cmd/ccr <某 profile 名前缀>` 唯一命中 → 直接拉起。
- 跑一次后 `~/.ccr/state.json` 出现 `last`；`go run ./cmd/ccr -` 重放上次。
- `go run ./cmd/ccr default <名字>` 后 `go run ./cmd/ccr .` 跑默认。

- [ ] **Step 5: 提交**

```bash
git add internal/cli/cli.go
git commit -m "feat: resolve - (last) / . (default) / fuzzy TUI and record last-used"
```

---

## Task 10: 更新帮助文本

**Files:**
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: 替换 printUsage**

把 `printUsage`（当前结尾的函数）整体替换为：

```go
// printUsage 打印帮助。
func printUsage(out io.Writer) {
	fmt.Fprint(out, `ccr — 用选定 provider 的环境变量启动 claude

用法:
  ccr                          交互式选择一个配置并启动
  ccr <名字|别名|前缀> [claude参数]  按名/别名/模糊命中启动，多余参数透传给 claude
  ccr -                        重跑上次用的配置
  ccr .                        跑默认配置（先 ccr default 设过）
  ccr ls                       列出所有配置（两来源）
  ccr show <名字> [--reveal]    查看某配置（默认 token 打码）
  ccr edit <名字>              用 $EDITOR 编辑/新建自定义配置
  ccr alias [<别名> <目标>]     列出 / 设置别名
  ccr unalias <别名>           删除别名
  ccr default [<名字>]          查看 / 设置默认配置
  ccr completion <shell>       打印补全脚本（bash/zsh/powershell）
  ccr completion install [shell] [--uninstall]
                               一键装/卸补全到当前 shell 配置

配置来源: cc-switch 库 + 自定义目录（~/.ccr/profiles/*.json）
元数据:   别名/默认存 ~/.ccr/overlay.json，上次用的存 ~/.ccr/state.json
`)
}
```

- [ ] **Step 2: 构建 + 冒烟**

Run: `go run ./cmd/ccr --help`
Expected: 打印含 alias/default/completion 的新帮助。

- [ ] **Step 3: 提交**

```bash
git add internal/cli/cli.go
git commit -m "docs: update ccr usage with alias/default/completion commands"
```

---

## Task 11: 提交前全套检查

**Files:** 无（验证）

- [ ] **Step 1: 跑 check**

Run: `task check`
Expected: fmt + vet + lint + test 全绿。若 lint 报未用/风格问题，按提示修后再跑。

- [ ] **Step 2: 若 check 改了文件则提交**

```bash
git add -A
git commit -m "chore: pass task check (fmt/vet/lint)"
```

---

## Task 12: README 文档

**Files:**
- Modify: `README.md`

- [ ] **Step 1: 加一节用法**

在 README 合适位置（用法/命令一节）补充别名、默认、上次、模糊命中、补全安装的说明。示例文案：

```markdown
### 别名 / 默认 / 上次

- `ccr alias prod cc-switch:DeepSeek` 设别名，之后 `ccr prod` 直启
- `ccr default my-local` 设默认，之后 `ccr .` 直启
- `ccr -` 重跑上次用的配置
- `ccr <前缀>` 模糊命中：唯一则直启，多个则弹选择器

别名/默认存 `~/.ccr/overlay.json`，上次用的存 `~/.ccr/state.json`。

### Shell 补全

```bash
# 一键安装到当前 shell（自动探测 bash/zsh/powershell）
ccr completion install
# 或显式指定，并可卸载
ccr completion install zsh
ccr completion install zsh --uninstall
# 也可手动取脚本自行放置
ccr completion bash > /path/to/place
```
```

- [ ] **Step 2: 提交**

```bash
git add README.md
git commit -m "docs: document alias/default/last and shell completion"
```

---

## Task 13（可选，低风险）: 在发布归档里附带补全脚本

**Files:**
- Modify: `.goreleaser.yaml`

> 背景：Homebrew 这里用的是 **Cask**（`homebrew_casks`），Scoop manifest 也无标准补全机制——都不支持 formula 那种「装包即装补全」。因此**主自动化路径是 `ccr completion install`**（与安装方式无关，通用）。本任务只是顺手把脚本塞进发布归档，方便手动放置，不承诺「装包即生效」。

- [ ] **Step 1: 生成脚本到 dist 并塞进归档**

在 `.goreleaser.yaml` 的 `before.hooks` 增加生成步骤，并在 `archives` 里附带文件。先加 hook：

```yaml
before:
  hooks:
    - go mod tidy
    - mkdir -p completions
    - sh -c 'go run ./cmd/ccr completion bash > completions/ccr.bash'
    - sh -c 'go run ./cmd/ccr completion zsh > completions/_ccr'
    - sh -c 'go run ./cmd/ccr completion powershell > completions/ccr.ps1'
```

在 `archives[0]` 下加：

```yaml
    files:
      - completions/*
```

并把 `completions/` 加进 `.gitignore`。

- [ ] **Step 2: 校验配置 + 本地试打包**

Run: `goreleaser check`
Expected: 配置合法。

Run: `task snapshot`
Expected: `dist/` 下归档内含 `completions/` 三个脚本。

- [ ] **Step 3: 提交**

```bash
git add .goreleaser.yaml .gitignore
git commit -m "build: bundle completion scripts in release archives"
```

---

## 收尾：发布与 PR

- 全部完成后，本批属 v0.2。按项目约定开分支推送、走 PR（master 受保护）。
- 发布说明由 GoReleaser 从 `feat:`/`fix:` commit 自动生成，无需手写 CHANGELOG。
- chain（v0.3）见 `docs/superpowers/specs/2026-06-09-ccr-chain-design.md`。
