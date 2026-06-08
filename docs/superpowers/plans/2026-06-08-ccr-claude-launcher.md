# ccr 跨平台 Claude 会话启动器 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现一条跨平台命令 `ccr`：把选定 provider 的环境变量注入当前终端并拉起 `claude`，从而能同时多开、各用不同后端，互不干扰，且不触碰全局配置。

**Architecture:** 三层——来源层（`ccswitch` 只读 SQLite + `customdir` 读 JSON 目录）→ 合并层（去重/标注来源/按名解析）→ 启动层（组装 env、按 model 追加参数、子进程拉起 claude 并透传退出码）。纯逻辑（解析/合并/env 组装/参数）全部单元测试；拉起子进程用 Go 标准的 helper-process 模式做集成测试；TUI 与 editor 拉起做手动验证。

**Tech Stack:** Go（`CGO_ENABLED=0`，三平台交叉编译单文件）；`modernc.org/sqlite`（纯 Go SQLite，免 cgo）；`github.com/charmbracelet/huh`（fuzzy 选择清单）。

**Module path:** 计划中 Go 模块名用 `ccr`，内部包为 `ccr/internal/...`。如需发布可改成完整路径（如 `github.com/<you>/ccr`）。

**参考规格：** `docs/superpowers/specs/2026-06-08-ccr-claude-launcher-design.md`

---

## 文件结构

```
cc-run/
  go.mod
  go.sum
  main.go                              # 瘦入口 → os.Exit(cli.Execute(os.Args[1:]))
  internal/
    profile/profile.go                 # Profile 结构、Source 常量、Redact 辅助
    profile/profile_test.go
    source/parse.go                    # settings_config(JSON) → Profile
    source/parse_test.go
    source/source.go                   # ProfileSource 接口 + LoadAll
    source/ccswitch.go                 # cc-switch 只读 SQLite 来源
    source/ccswitch_test.go
    source/customdir.go                # 自定义目录 JSON 来源
    source/customdir_test.go
    config/config.go                   # 路径解析：env > ~/.ccr/config.json > 默认
    config/config_test.go
    registry/registry.go               # 合并、去重、按名解析、建议
    registry/registry_test.go
    launcher/launcher.go               # ComposeEnv / ClaudeArgs / Launcher.Run
    launcher/launcher_test.go
    tui/select.go                      # huh fuzzy 选择器（手动验证）
    cli/cli.go                         # 参数分发：ls/show/edit/<name>/交互
    cli/cli_test.go
  README.md
```

每个文件单一职责：来源各管一种数据；`registry` 只管合并与解析；`launcher` 只管"拿一个 Profile 起 claude"；`cli` 只做参数路由与装配。

---

## Task 0: 工具链与项目骨架

**Files:**
- Create: `go.mod`
- Create: `main.go`

- [ ] **Step 1: 安装 Go（用户已装 mise）**

Run（PowerShell）:
```
mise use -g go@latest
go version
```
Expected: 打印 `go version go1.2x ...`。若 `mise` 装的 go 不在 PATH，运行 `mise reshim` 后重开终端，或改用 `scoop install go`。

- [ ] **Step 2: 初始化模块**

Run（在 `D:\owner\cc-run`）:
```
go mod init ccr
```
Expected: 生成 `go.mod`，内容含 `module ccr` 与 `go 1.2x`。

- [ ] **Step 3: 写最小入口 `main.go`**

```go
package main

import "fmt"

func main() {
	fmt.Println("ccr: ok")
}
```

- [ ] **Step 4: 构建并运行**

Run:
```
go build -o ccr.exe . ; ./ccr.exe
```
Expected: 打印 `ccr: ok`。

- [ ] **Step 5: 加 .gitignore 并提交**

Create `.gitignore`:
```
/ccr
/ccr.exe
/dist/
```

```bash
git add go.mod main.go .gitignore
git commit -m "chore: 初始化 ccr Go 项目骨架"
```

---

## Task 1: Profile 类型与 token 打码

**Files:**
- Create: `internal/profile/profile.go`
- Test: `internal/profile/profile_test.go`

- [ ] **Step 1: 写失败测试**

`internal/profile/profile_test.go`:
```go
package profile

import "testing"

func TestRedactToken(t *testing.T) {
	cases := map[string]string{
		"sk-FAKE000000000000000000000000000": "sk-FAKE…",
		"ark-FAKE0000":                       "ark-882…",
		"短":                                  "…",
		"":                                   "",
	}
	for in, want := range cases {
		if got := RedactToken(in); got != want {
			t.Errorf("RedactToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactEnvHidesSecrets(t *testing.T) {
	env := map[string]string{
		"ANTHROPIC_AUTH_TOKEN": "sk-FAKE000000000000000000000000000",
		"ANTHROPIC_BASE_URL":   "https://api.deepseek.com/anthropic",
	}
	out := RedactEnv(env)
	if out["ANTHROPIC_AUTH_TOKEN"] != "sk-8035…" {
		t.Errorf("token 未打码: %q", out["ANTHROPIC_AUTH_TOKEN"])
	}
	if out["ANTHROPIC_BASE_URL"] != "https://api.deepseek.com/anthropic" {
		t.Errorf("非密钥被改动: %q", out["ANTHROPIC_BASE_URL"])
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/profile/ -run Redact -v`
Expected: FAIL（`undefined: RedactToken`）。

- [ ] **Step 3: 写实现**

`internal/profile/profile.go`:
```go
// Package profile 定义可启动的 Claude 配置及其脱敏辅助。
package profile

import "strings"

// Source 标识一个 Profile 的来源。
type Source string

const (
	SourceCCSwitch Source = "cc-switch"
	SourceCustom   Source = "custom"
)

// Profile 是一份可启动的 Claude 配置。
type Profile struct {
	Name      string            // provider name 或自定义文件名(去扩展名)
	Source    Source            // 来源
	Model     string            // settings_config 顶层 "model"，可空
	Env       map[string]string // 注入 claude 的环境变量
	BaseURL   string            // Env["ANTHROPIC_BASE_URL"]，仅展示用
	IsCurrent bool              // 仅 cc-switch：是否为当前激活 provider
}

// secretKeySubstrings 命中其一即视为敏感值，需打码。
var secretKeySubstrings = []string{"TOKEN", "KEY", "SECRET", "PASSWORD"}

// RedactToken 保留前 7 个字符，其余用省略号代替。
func RedactToken(s string) string {
	if s == "" {
		return ""
	}
	const keep = 7
	r := []rune(s)
	if len(r) <= keep {
		return "…"
	}
	return string(r[:keep]) + "…"
}

// isSecretKey 判断某环境变量名是否为敏感项。
func isSecretKey(k string) bool {
	up := strings.ToUpper(k)
	for _, s := range secretKeySubstrings {
		if strings.Contains(up, s) {
			return true
		}
	}
	return false
}

// RedactEnv 返回一份副本，其中敏感项的值被打码。
func RedactEnv(env map[string]string) map[string]string {
	out := make(map[string]string, len(env))
	for k, v := range env {
		if isSecretKey(k) {
			out[k] = RedactToken(v)
		} else {
			out[k] = v
		}
	}
	return out
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/profile/ -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/profile/
git commit -m "feat(profile): Profile 类型与 token 打码辅助"
```

---

## Task 2: settings_config 解析

把 cc-switch 和自定义文件共用的 JSON（`{"model":..., "env":{...}}`）解析为 `Profile`。

**Files:**
- Create: `internal/source/parse.go`
- Test: `internal/source/parse_test.go`

- [ ] **Step 1: 写失败测试（用真实样本）**

`internal/source/parse_test.go`:
```go
package source

import (
	"testing"

	"ccr/internal/profile"
)

func TestParseSettingsConfig_火山(t *testing.T) {
	raw := `{"model":"sonnet","env":{"ANTHROPIC_AUTH_TOKEN":"ark-FAKE0000","ANTHROPIC_BASE_URL":"https://ark.cn-beijing.volces.com/api/coding","ANTHROPIC_DEFAULT_OPUS_MODEL":"glm-5.1"}}`
	p, err := ParseSettingsConfig("火山 Coding Plan", profile.SourceCCSwitch, raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if p.Name != "火山 Coding Plan" || p.Source != profile.SourceCCSwitch {
		t.Errorf("名字/来源错误: %+v", p)
	}
	if p.Model != "sonnet" {
		t.Errorf("Model = %q, want sonnet", p.Model)
	}
	if p.Env["ANTHROPIC_DEFAULT_OPUS_MODEL"] != "glm-5.1" {
		t.Errorf("env 缺失: %+v", p.Env)
	}
	if p.BaseURL != "https://ark.cn-beijing.volces.com/api/coding" {
		t.Errorf("BaseURL = %q", p.BaseURL)
	}
}

func TestParseSettingsConfig_DeepSeek无顶层model(t *testing.T) {
	raw := `{"env":{"ANTHROPIC_BASE_URL":"https://api.deepseek.com/anthropic","ANTHROPIC_MODEL":"deepseek-v4-pro"}}`
	p, err := ParseSettingsConfig("DeepSeek", profile.SourceCustom, raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if p.Model != "" {
		t.Errorf("Model 应为空, got %q", p.Model)
	}
	if p.Env["ANTHROPIC_MODEL"] != "deepseek-v4-pro" {
		t.Errorf("env 错误: %+v", p.Env)
	}
}

func TestParseSettingsConfig_无env也不报错(t *testing.T) {
	p, err := ParseSettingsConfig("default", profile.SourceCCSwitch, `{"model":"opus"}`)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if p.Env == nil {
		t.Error("Env 应初始化为空 map 而非 nil")
	}
}

func TestParseSettingsConfig_坏JSON报错(t *testing.T) {
	if _, err := ParseSettingsConfig("bad", profile.SourceCustom, `{not json`); err == nil {
		t.Error("坏 JSON 应返回错误")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/source/ -run ParseSettingsConfig -v`
Expected: FAIL（`undefined: ParseSettingsConfig`）。

- [ ] **Step 3: 写实现**

`internal/source/parse.go`:
```go
// Package source 把各来源的原始数据解析为 profile.Profile。
package source

import (
	"encoding/json"
	"fmt"

	"ccr/internal/profile"
)

// settingsConfig 对应 cc-switch settings_config 与自定义文件的 JSON 形状。
type settingsConfig struct {
	Model string            `json:"model"`
	Env   map[string]string `json:"env"`
}

// ParseSettingsConfig 将一段 settings_config JSON 解析为 Profile。
func ParseSettingsConfig(name string, src profile.Source, raw string) (profile.Profile, error) {
	var sc settingsConfig
	if err := json.Unmarshal([]byte(raw), &sc); err != nil {
		return profile.Profile{}, fmt.Errorf("解析 %q 的配置失败: %w", name, err)
	}
	if sc.Env == nil {
		sc.Env = map[string]string{}
	}
	return profile.Profile{
		Name:    name,
		Source:  src,
		Model:   sc.Model,
		Env:     sc.Env,
		BaseURL: sc.Env["ANTHROPIC_BASE_URL"],
	}, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/source/ -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/source/parse.go internal/source/parse_test.go
git commit -m "feat(source): settings_config JSON 解析"
```

---

## Task 3: 来源接口与自定义目录来源

**Files:**
- Create: `internal/source/source.go`
- Create: `internal/source/customdir.go`
- Test: `internal/source/customdir_test.go`

- [ ] **Step 1: 写接口**

`internal/source/source.go`:
```go
package source

import "ccr/internal/profile"

// ProfileSource 是一个 Profile 来源。
type ProfileSource interface {
	// Available 报告该来源是否存在（如库文件/目录是否在）。
	Available() bool
	// Load 解析并返回该来源的所有 Profile。
	Load() ([]profile.Profile, error)
}

// LoadAll 加载所有 Available 的来源，汇总 Profile 与各自的错误（错误不致命）。
func LoadAll(srcs ...ProfileSource) ([]profile.Profile, []error) {
	var all []profile.Profile
	var errs []error
	for _, s := range srcs {
		if !s.Available() {
			continue
		}
		ps, err := s.Load()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		all = append(all, ps...)
	}
	return all, errs
}
```

- [ ] **Step 2: 写失败测试**

`internal/source/customdir_test.go`:
```go
package source

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCustomDir_LoadsJSONFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "deepseek.json", `{"env":{"ANTHROPIC_BASE_URL":"https://api.deepseek.com/anthropic"}}`)
	writeFile(t, dir, "notes.txt", `ignore me`)

	src := NewCustomDir(dir)
	if !src.Available() {
		t.Fatal("目录存在时 Available 应为 true")
	}
	ps, err := src.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 {
		t.Fatalf("应只加载 1 个 .json, got %d", len(ps))
	}
	if ps[0].Name != "deepseek" {
		t.Errorf("Name 应取文件名去扩展名, got %q", ps[0].Name)
	}
	if ps[0].Source != "custom" {
		t.Errorf("Source = %q", ps[0].Source)
	}
}

func TestCustomDir_坏文件跳过其余照常(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good.json", `{"env":{}}`)
	writeFile(t, dir, "bad.json", `{not json`)

	ps, err := NewCustomDir(dir).Load()
	if err != nil {
		t.Fatalf("坏文件不应整体报错: %v", err)
	}
	if len(ps) != 1 || ps[0].Name != "good" {
		t.Fatalf("应只加载 good, got %+v", ps)
	}
}

func TestCustomDir_目录不存在(t *testing.T) {
	src := NewCustomDir(filepath.Join(t.TempDir(), "nope"))
	if src.Available() {
		t.Error("不存在的目录 Available 应为 false")
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/source/ -run CustomDir -v`
Expected: FAIL（`undefined: NewCustomDir`）。

- [ ] **Step 4: 写实现**

`internal/source/customdir.go`:
```go
package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ccr/internal/profile"
)

// CustomDir 从一个目录读取 *.json，每个文件一个 Profile。
type CustomDir struct {
	Dir string
	// Warnf 处理坏文件告警，默认写 stderr；测试可替换。
	Warnf func(format string, a ...any)
}

// NewCustomDir 构造一个自定义目录来源。
func NewCustomDir(dir string) *CustomDir {
	return &CustomDir{
		Dir:   dir,
		Warnf: func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) },
	}
}

// Available 报告目录是否存在。
func (c *CustomDir) Available() bool {
	info, err := os.Stat(c.Dir)
	return err == nil && info.IsDir()
}

// Load 读取目录下所有 *.json；坏文件告警并跳过。
func (c *CustomDir) Load() ([]profile.Profile, error) {
	matches, err := filepath.Glob(filepath.Join(c.Dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var out []profile.Profile
	for _, path := range matches {
		raw, err := os.ReadFile(path)
		if err != nil {
			c.Warnf("跳过 %s: %v", path, err)
			continue
		}
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		p, err := ParseSettingsConfig(name, profile.SourceCustom, string(raw))
		if err != nil {
			c.Warnf("跳过 %s: %v", path, err)
			continue
		}
		out = append(out, p)
	}
	return out, nil
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/source/ -v`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/source/source.go internal/source/customdir.go internal/source/customdir_test.go
git commit -m "feat(source): ProfileSource 接口与自定义目录来源"
```

---

## Task 4: cc-switch 只读 SQLite 来源

**Files:**
- Create: `internal/source/ccswitch.go`
- Test: `internal/source/ccswitch_test.go`
- Modify: `go.mod`（加 `modernc.org/sqlite`）

- [ ] **Step 1: 加 SQLite 依赖**

Run:
```
go get modernc.org/sqlite@latest
```
Expected: `go.mod` 出现 `require modernc.org/sqlite ...`。

- [ ] **Step 2: 写失败测试（用临时 sqlite fixture）**

`internal/source/ccswitch_test.go`:
```go
package source

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// newFixtureDB 在临时目录建一个含 providers 表的可写库并插入若干行。
func newFixtureDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cc-switch.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE providers (
		id TEXT, app_type TEXT, name TEXT, settings_config TEXT, is_current INTEGER DEFAULT 0
	)`); err != nil {
		t.Fatal(err)
	}
	rows := []struct {
		app, name, cfg string
		cur            int
	}{
		{"claude", "default", `{"model":"opus"}`, 1},
		{"claude", "DeepSeek", `{"env":{"ANTHROPIC_BASE_URL":"https://api.deepseek.com/anthropic"}}`, 0},
		{"codex", "OpenAI", `{"auth":{}}`, 0}, // 非 claude，应被过滤
	}
	for i, r := range rows {
		if _, err := db.Exec(
			`INSERT INTO providers(id,app_type,name,settings_config,is_current) VALUES(?,?,?,?,?)`,
			i, r.app, r.name, r.cfg, r.cur,
		); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestCCSwitch_只加载claude且解析正确(t *testing.T) {
	path := newFixtureDB(t)
	src := NewCCSwitch(path)
	if !src.Available() {
		t.Fatal("库存在时 Available 应为 true")
	}
	ps, err := src.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Fatalf("应只加载 2 个 claude provider, got %d (%+v)", len(ps), ps)
	}
	byName := map[string]bool{}
	for _, p := range ps {
		byName[p.Name] = p.IsCurrent
		if p.Source != "cc-switch" {
			t.Errorf("Source = %q", p.Source)
		}
	}
	if !byName["default"] {
		t.Error("default 的 IsCurrent 应为 true")
	}
	if byName["DeepSeek"] {
		t.Error("DeepSeek 的 IsCurrent 应为 false")
	}
}

func TestCCSwitch_库不存在(t *testing.T) {
	src := NewCCSwitch(filepath.Join(t.TempDir(), "nope.db"))
	if src.Available() {
		t.Error("库不存在时 Available 应为 false")
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/source/ -run CCSwitch -v`
Expected: FAIL（`undefined: NewCCSwitch`）。

- [ ] **Step 4: 写实现**

`internal/source/ccswitch.go`:
```go
package source

import (
	"database/sql"
	"fmt"
	"os"

	"ccr/internal/profile"

	_ "modernc.org/sqlite"
)

// CCSwitch 以只读方式从 cc-switch 的 SQLite 库读取 claude provider。
type CCSwitch struct {
	Path string
}

// NewCCSwitch 构造一个 cc-switch 来源。
func NewCCSwitch(path string) *CCSwitch {
	return &CCSwitch{Path: path}
}

// Available 报告库文件是否存在。
func (c *CCSwitch) Available() bool {
	info, err := os.Stat(c.Path)
	return err == nil && !info.IsDir()
}

// Load 只读打开库，读取 app_type='claude' 的 provider。
func (c *CCSwitch) Load() ([]profile.Profile, error) {
	dsn := "file:" + c.Path + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 cc-switch 库失败: %w", err)
	}
	defer db.Close()
	// 只读期间避免长时间等待写锁。
	_, _ = db.Exec("PRAGMA busy_timeout=2000")

	rows, err := db.Query(
		`SELECT name, settings_config, COALESCE(is_current,0) FROM providers WHERE app_type='claude'`,
	)
	if err != nil {
		return nil, fmt.Errorf("查询 providers 失败: %w", err)
	}
	defer rows.Close()

	var out []profile.Profile
	for rows.Next() {
		var name, cfg string
		var isCur int
		if err := rows.Scan(&name, &cfg, &isCur); err != nil {
			return nil, err
		}
		p, err := ParseSettingsConfig(name, profile.SourceCCSwitch, cfg)
		if err != nil {
			// 单条坏数据不致命：跳过。
			continue
		}
		p.IsCurrent = isCur != 0
		out = append(out, p)
	}
	return out, rows.Err()
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/source/ -v`
Expected: PASS（首次会编译 modernc.org/sqlite，稍慢）。

- [ ] **Step 6: 提交**

```bash
git add internal/source/ccswitch.go internal/source/ccswitch_test.go go.mod go.sum
git commit -m "feat(source): cc-switch 只读 SQLite 来源"
```

---

## Task 5: 配置与路径解析

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

解析优先级：环境变量 > `~/.ccr/config.json` > 默认值。

- [ ] **Step 1: 写失败测试**

`internal/config/config_test.go`:
```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_默认值基于home(t *testing.T) {
	home := t.TempDir()
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = os.UserHomeDir })
	t.Setenv("CCR_DB", "")
	t.Setenv("CCR_PROFILES_DIR", "")

	c := Load()
	if c.DB != filepath.Join(home, ".cc-switch", "cc-switch.db") {
		t.Errorf("DB 默认值错误: %q", c.DB)
	}
	if c.ProfilesDir != filepath.Join(home, ".ccr", "profiles") {
		t.Errorf("ProfilesDir 默认值错误: %q", c.ProfilesDir)
	}
}

func TestLoad_环境变量覆盖(t *testing.T) {
	home := t.TempDir()
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = os.UserHomeDir })
	t.Setenv("CCR_DB", "/x/my.db")
	t.Setenv("CCR_PROFILES_DIR", "/x/profiles")

	c := Load()
	if c.DB != "/x/my.db" || c.ProfilesDir != "/x/profiles" {
		t.Errorf("env 未覆盖: %+v", c)
	}
}

func TestLoad_配置文件覆盖默认但低于env(t *testing.T) {
	home := t.TempDir()
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = os.UserHomeDir })
	if err := os.MkdirAll(filepath.Join(home, ".ccr"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(home, ".ccr", "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"db":"/from/file.db","profilesDir":"/from/file/profiles"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CCR_DB", "")            // 不设 → 用文件值
	t.Setenv("CCR_PROFILES_DIR", "/env/wins") // 设了 → 覆盖文件

	c := Load()
	if c.DB != "/from/file.db" {
		t.Errorf("DB 应取文件值, got %q", c.DB)
	}
	if c.ProfilesDir != "/env/wins" {
		t.Errorf("ProfilesDir 应取 env 值, got %q", c.ProfilesDir)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/config/ -v`
Expected: FAIL（`undefined: Load` / `userHomeDir`）。

- [ ] **Step 3: 写实现**

`internal/config/config.go`:
```go
// Package config 解析 ccr 的路径配置：env > ~/.ccr/config.json > 默认。
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// userHomeDir 便于测试替换。
var userHomeDir = os.UserHomeDir

// Config 是解析后的运行时路径。
type Config struct {
	DB          string // cc-switch 库路径
	ProfilesDir string // 自定义 profiles 目录
}

// fileConfig 对应 ~/.ccr/config.json。
type fileConfig struct {
	DB          string `json:"db"`
	ProfilesDir string `json:"profilesDir"`
}

// Load 按优先级解析配置。任何缺失项回退到下一级。
func Load() Config {
	home, err := userHomeDir()
	if err != nil {
		home = "."
	}
	defDB := filepath.Join(home, ".cc-switch", "cc-switch.db")
	defProfiles := filepath.Join(home, ".ccr", "profiles")

	var fc fileConfig
	if raw, err := os.ReadFile(filepath.Join(home, ".ccr", "config.json")); err == nil {
		_ = json.Unmarshal(raw, &fc) // 坏文件忽略，回退默认
	}

	pick := func(envKey, fileVal, def string) string {
		if v := os.Getenv(envKey); v != "" {
			return v
		}
		if fileVal != "" {
			return fileVal
		}
		return def
	}

	return Config{
		DB:          pick("CCR_DB", fc.DB, defDB),
		ProfilesDir: pick("CCR_PROFILES_DIR", fc.ProfilesDir, defProfiles),
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/config/ -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/config/
git commit -m "feat(config): 路径解析 env>配置文件>默认"
```

---

## Task 6: 合并层（registry）

**Files:**
- Create: `internal/registry/registry.go`
- Test: `internal/registry/registry_test.go`

- [ ] **Step 1: 写失败测试**

`internal/registry/registry_test.go`:
```go
package registry

import (
	"strings"
	"testing"

	"ccr/internal/profile"
)

func sample() []profile.Profile {
	return []profile.Profile{
		{Name: "DeepSeek", Source: profile.SourceCCSwitch},
		{Name: "火山", Source: profile.SourceCCSwitch},
		{Name: "DeepSeek", Source: profile.SourceCustom}, // 与 cc-switch 同名
		{Name: "my-local", Source: profile.SourceCustom},
	}
}

func TestResolve_唯一匹配(t *testing.T) {
	r := New(sample())
	p, err := r.Resolve("火山")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "火山" {
		t.Errorf("got %q", p.Name)
	}
}

func TestResolve_重名歧义报错(t *testing.T) {
	r := New(sample())
	_, err := r.Resolve("DeepSeek")
	if err == nil {
		t.Fatal("重名应报歧义错误")
	}
	if !strings.Contains(err.Error(), "cc-switch:DeepSeek") ||
		!strings.Contains(err.Error(), "custom:DeepSeek") {
		t.Errorf("歧义错误应给出限定名: %v", err)
	}
}

func TestResolve_限定名消歧(t *testing.T) {
	r := New(sample())
	p, err := r.Resolve("custom:DeepSeek")
	if err != nil {
		t.Fatal(err)
	}
	if p.Source != profile.SourceCustom {
		t.Errorf("应取 custom, got %q", p.Source)
	}
}

func TestResolve_未命中给建议(t *testing.T) {
	r := New(sample())
	_, err := r.Resolve("deep")
	if err == nil {
		t.Fatal("未精确命中应报错")
	}
	if !strings.Contains(err.Error(), "DeepSeek") {
		t.Errorf("应建议含子串的名字: %v", err)
	}
}

func TestList_返回全部(t *testing.T) {
	r := New(sample())
	if len(r.List()) != 4 {
		t.Errorf("List 应返回全部 4 个")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/registry/ -v`
Expected: FAIL（`undefined: New`）。

- [ ] **Step 3: 写实现**

`internal/registry/registry.go`:
```go
// Package registry 合并多来源的 Profile 并按名解析。
package registry

import (
	"fmt"
	"sort"
	"strings"

	"ccr/internal/profile"
)

// Registry 持有合并后的 Profile 列表。
type Registry struct {
	profiles []profile.Profile
}

// New 用一组 Profile 构造 Registry。
func New(profiles []profile.Profile) *Registry {
	return &Registry{profiles: profiles}
}

// List 返回全部 Profile（按 来源、名字 稳定排序）。
func (r *Registry) List() []profile.Profile {
	out := make([]profile.Profile, len(r.profiles))
	copy(out, r.profiles)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Resolve 按 query 找唯一 Profile。query 可为 "name" 或 "source:name"。
func (r *Registry) Resolve(query string) (profile.Profile, error) {
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

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return profile.Profile{}, r.notFoundErr(name)
	default:
		var qualified []string
		for _, p := range matches {
			qualified = append(qualified, fmt.Sprintf("%s:%s", p.Source, p.Name))
		}
		return profile.Profile{}, fmt.Errorf(
			"名字 %q 有多个来源，请用限定名指定其一：%s",
			name, strings.Join(qualified, " 、 "),
		)
	}
}

// splitQuery 拆出可选的 "source:" 前缀。
func splitQuery(q string) (source, name string) {
	if i := strings.IndexByte(q, ':'); i > 0 {
		prefix := q[:i]
		if prefix == string(profile.SourceCCSwitch) || prefix == string(profile.SourceCustom) {
			return prefix, q[i+1:]
		}
	}
	return "", q
}

// notFoundErr 构造带建议的未命中错误。
func (r *Registry) notFoundErr(name string) error {
	var suggestions []string
	lower := strings.ToLower(name)
	for _, p := range r.profiles {
		if strings.Contains(strings.ToLower(p.Name), lower) {
			suggestions = append(suggestions, p.Name)
		}
	}
	if len(suggestions) == 0 {
		return fmt.Errorf("找不到名为 %q 的配置；用 `ccr ls` 查看全部", name)
	}
	return fmt.Errorf("找不到 %q，你是不是指：%s", name, strings.Join(suggestions, " 、 "))
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/registry/ -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/registry/
git commit -m "feat(registry): 多来源合并与按名解析"
```

---

## Task 7: 启动层（env 组装 / 参数 / 拉起 claude）

**Files:**
- Create: `internal/launcher/launcher.go`
- Test: `internal/launcher/launcher_test.go`

纯逻辑（`ComposeEnv`/`ClaudeArgs`）单元测试；拉起子进程用 helper-process 模式集成测试。

- [ ] **Step 1: 写失败测试（纯逻辑 + helper-process 集成）**

`internal/launcher/launcher_test.go`:
```go
package launcher

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"ccr/internal/profile"
)

func TestComposeEnv_profile覆盖且去重(t *testing.T) {
	base := []string{"PATH=/bin", "ANTHROPIC_BASE_URL=https://old"}
	env := map[string]string{"ANTHROPIC_BASE_URL": "https://new", "FOO": "bar"}
	got := ComposeEnv(base, env)

	found := map[string]string{}
	for _, kv := range got {
		i := strings.IndexByte(kv, '=')
		found[kv[:i]] = kv[i+1:]
	}
	if found["ANTHROPIC_BASE_URL"] != "https://new" {
		t.Errorf("profile 应覆盖 base: %q", found["ANTHROPIC_BASE_URL"])
	}
	if found["PATH"] != "/bin" || found["FOO"] != "bar" {
		t.Errorf("base/新增项丢失: %+v", found)
	}
	// 去重：BASE_URL 只应出现一次
	n := 0
	for _, kv := range got {
		if strings.HasPrefix(kv, "ANTHROPIC_BASE_URL=") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("ANTHROPIC_BASE_URL 出现 %d 次，应为 1", n)
	}
}

func TestClaudeArgs(t *testing.T) {
	// 有顶层 model 且 env 未指定模型 → 追加 --model
	got := ClaudeArgs("sonnet", nil, []string{"-p", "hi"})
	if strings.Join(got, " ") != "--model sonnet -p hi" {
		t.Errorf("got %v", got)
	}
	// env 已含 ANTHROPIC_MODEL → 不追加
	got = ClaudeArgs("sonnet", map[string]string{"ANTHROPIC_MODEL": "x"}, []string{"-p"})
	if strings.Join(got, " ") != "-p" {
		t.Errorf("env 指定模型时不应追加 --model, got %v", got)
	}
	// 无顶层 model → 不追加
	got = ClaudeArgs("", nil, []string{"chat"})
	if strings.Join(got, " ") != "chat" {
		t.Errorf("got %v", got)
	}
}

// TestHelperProcess 充当假的 claude：打印某环境变量并以特定码退出。
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Stdout.WriteString("BASE=" + os.Getenv("ANTHROPIC_BASE_URL"))
	os.Exit(7)
}

func TestRun_注入env并透传退出码(t *testing.T) {
	l := New()
	l.ClaudePath = os.Args[0] // 用测试二进制自身当作 claude
	l.Environ = func() []string { return append(os.Environ(), "GO_WANT_HELPER_PROCESS=1") }
	var out bytes.Buffer
	l.Stdout = &out

	p := profile.Profile{Env: map[string]string{"ANTHROPIC_BASE_URL": "https://injected"}}
	code, err := l.Run(p, []string{"-test.run=TestHelperProcess"})
	if err != nil {
		t.Fatal(err)
	}
	if code != 7 {
		t.Errorf("退出码 = %d, want 7", code)
	}
	if !strings.Contains(out.String(), "BASE=https://injected") {
		t.Errorf("env 未注入子进程, 输出: %q", out.String())
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/launcher/ -v`
Expected: FAIL（`undefined: ComposeEnv` 等）。

- [ ] **Step 3: 写实现**

`internal/launcher/launcher.go`:
```go
// Package launcher 用某个 Profile 拉起 claude 子进程。
package launcher

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"ccr/internal/profile"
)

// ComposeEnv 把 profileEnv 叠加到 base 上（profile 覆盖同名键），去重后返回。
func ComposeEnv(base []string, profileEnv map[string]string) []string {
	merged := map[string]string{}
	for _, kv := range base {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			merged[kv[:i]] = kv[i+1:]
		}
	}
	for k, v := range profileEnv {
		merged[k] = v
	}
	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	sort.Strings(out) // 稳定输出，便于测试与调试
	return out
}

// ClaudeArgs 计算传给 claude 的参数。
// 仅当存在顶层 model 且 env 未通过 ANTHROPIC_MODEL 指定模型时，追加 --model。
func ClaudeArgs(model string, profileEnv map[string]string, extra []string) []string {
	var args []string
	_, envHasModel := profileEnv["ANTHROPIC_MODEL"]
	if model != "" && !envHasModel {
		args = append(args, "--model", model)
	}
	return append(args, extra...)
}

// Launcher 拉起 claude；各字段可注入以便测试。
type Launcher struct {
	ClaudePath string                       // 为空则在 PATH 中查找 "claude"
	LookPath   func(string) (string, error) // 默认 exec.LookPath
	Environ    func() []string              // 默认 os.Environ
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
}

// New 返回带默认值的 Launcher。
func New() *Launcher {
	return &Launcher{
		LookPath: exec.LookPath,
		Environ:  os.Environ,
		Stdin:    os.Stdin,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
	}
}

// Run 用 p 的 env 拉起 claude，透传 stdio 与退出码。
// 返回子进程退出码；仅当无法启动时返回非 nil error。
func (l *Launcher) Run(p profile.Profile, extra []string) (int, error) {
	path := l.ClaudePath
	if path == "" {
		found, err := l.LookPath("claude")
		if err != nil {
			return -1, fmt.Errorf("找不到 claude 可执行文件，请确认已安装且在 PATH 中：%w", err)
		}
		path = found
	}

	cmd := exec.Command(path, ClaudeArgs(p.Model, p.Env, extra)...)
	cmd.Env = ComposeEnv(l.Environ(), p.Env)
	cmd.Stdin = l.Stdin
	cmd.Stdout = l.Stdout
	cmd.Stderr = l.Stderr

	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if ok := asExitError(err, &exitErr); ok {
		return exitErr.ExitCode(), nil // 子进程正常退出但非 0：透传，不算 ccr 错误
	}
	return -1, err
}

// asExitError 是 errors.As 的小封装，便于阅读。
func asExitError(err error, target **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*target = e
	}
	return ok
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/launcher/ -v`
Expected: PASS（含 `TestRun_注入env并透传退出码`）。

- [ ] **Step 5: 提交**

```bash
git add internal/launcher/
git commit -m "feat(launcher): env 组装、claude 参数与子进程拉起"
```

---

## Task 8: TUI 选择器（huh）

无参数时弹出的 fuzzy 列表。UI 代码做手动验证。

**Files:**
- Create: `internal/tui/select.go`
- Modify: `go.mod`（加 `github.com/charmbracelet/huh`）

- [ ] **Step 1: 加依赖**

Run:
```
go get github.com/charmbracelet/huh@latest
```
Expected: `go.mod` 出现 `github.com/charmbracelet/huh`。

- [ ] **Step 2: 写实现**

`internal/tui/select.go`:
```go
// Package tui 提供交互式 Profile 选择。
package tui

import (
	"fmt"

	"ccr/internal/profile"

	"github.com/charmbracelet/huh"
)

// SelectProfile 弹出可过滤的清单，返回用户选中的 Profile。
// 取消选择（Esc/Ctrl-C）时返回 huh.ErrUserAborted。
func SelectProfile(profiles []profile.Profile) (profile.Profile, error) {
	if len(profiles) == 0 {
		return profile.Profile{}, fmt.Errorf("没有可用的 profile")
	}

	opts := make([]huh.Option[int], 0, len(profiles))
	for i, p := range profiles {
		opts = append(opts, huh.NewOption(label(p), i))
	}

	var idx int
	field := huh.NewSelect[int]().
		Title("选择一个 Claude 配置启动").
		Options(opts...).
		Filtering(true).
		Value(&idx)

	if err := huh.NewForm(huh.NewGroup(field)).Run(); err != nil {
		return profile.Profile{}, err
	}
	return profiles[idx], nil
}

// label 渲染一行：名字 [来源] (模型) — 主机名，当前项加 ●。
func label(p profile.Profile) string {
	cur := ""
	if p.IsCurrent {
		cur = " ●"
	}
	model := ""
	if p.Model != "" {
		model = " (" + p.Model + ")"
	}
	host := ""
	if p.BaseURL != "" {
		host = " — " + p.BaseURL
	}
	return fmt.Sprintf("%s%s  [%s]%s%s", p.Name, cur, p.Source, model, host)
}
```

- [ ] **Step 3: 编译确认无误**

Run: `go build ./...`
Expected: 无错误。

- [ ] **Step 4: 提交**

```bash
git add internal/tui/ go.mod go.sum
git commit -m "feat(tui): huh fuzzy 选择器"
```

> 注：交互体验在 Task 9 接好 CLI 后做端到端手动验证。

---

## Task 9: CLI 装配（参数分发）

把各层接到一起，处理 `ccr` / `ccr <name>` / `ccr ls` / `ccr show` / `ccr edit`。

**Files:**
- Create: `internal/cli/cli.go`
- Test: `internal/cli/cli_test.go`
- Modify: `main.go`

- [ ] **Step 1: 写失败测试（可测的部分：ls / show / 未知名字）**

`internal/cli/cli_test.go`:
```go
package cli

import (
	"bytes"
	"strings"
	"testing"

	"ccr/internal/profile"
	"ccr/internal/registry"
)

func reg() *registry.Registry {
	return registry.New([]profile.Profile{
		{Name: "DeepSeek", Source: profile.SourceCustom, Model: "sonnet",
			Env: map[string]string{"ANTHROPIC_AUTH_TOKEN": "sk-1234567890", "ANTHROPIC_BASE_URL": "https://api.deepseek.com/anthropic"},
			BaseURL: "https://api.deepseek.com/anthropic"},
	})
}

func TestCmdLs_列出且不泄露token(t *testing.T) {
	var out bytes.Buffer
	code := cmdLs(reg(), &out)
	if code != 0 {
		t.Fatalf("退出码 %d", code)
	}
	s := out.String()
	if !strings.Contains(s, "DeepSeek") || !strings.Contains(s, "custom") {
		t.Errorf("ls 输出缺名字/来源: %q", s)
	}
	if strings.Contains(s, "sk-1234567890") {
		t.Errorf("ls 不应出现完整 token: %q", s)
	}
}

func TestCmdShow_默认打码(t *testing.T) {
	var out bytes.Buffer
	code := cmdShow(reg(), "DeepSeek", false, &out)
	if code != 0 {
		t.Fatalf("退出码 %d", code)
	}
	s := out.String()
	if strings.Contains(s, "sk-1234567890") {
		t.Errorf("默认 show 不应出现完整 token: %q", s)
	}
	if !strings.Contains(s, "sk-1234…") {
		t.Errorf("show 应出现打码 token: %q", s)
	}
}

func TestCmdShow_reveal显示完整(t *testing.T) {
	var out bytes.Buffer
	cmdShow(reg(), "DeepSeek", true, &out)
	if !strings.Contains(out.String(), "sk-1234567890") {
		t.Errorf("--reveal 应显示完整 token")
	}
}

func TestCmdShow_未知名字非零退出(t *testing.T) {
	var out bytes.Buffer
	if code := cmdShow(reg(), "nope", false, &out); code == 0 {
		t.Error("未知名字应非零退出")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/cli/ -v`
Expected: FAIL（`undefined: cmdLs` / `cmdShow`）。

- [ ] **Step 3: 写实现**

`internal/cli/cli.go`:
```go
// Package cli 解析参数并把各层装配起来。
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"

	"ccr/internal/config"
	"ccr/internal/launcher"
	"ccr/internal/profile"
	"ccr/internal/registry"
	"ccr/internal/source"
	"ccr/internal/tui"
)

// Execute 是 CLI 入口，返回进程退出码。
func Execute(args []string) int {
	cfg := config.Load()

	// 子命令分发。
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help":
			printUsage(os.Stdout)
			return 0
		case "ls":
			r, code := buildRegistry(cfg)
			if code != 0 {
				return code
			}
			return cmdLs(r, os.Stdout)
		case "show":
			return runShow(cfg, args[1:])
		case "edit":
			return runEdit(cfg, args[1:])
		}
	}

	// 其余：ccr <name> [claude 参数...] 或 ccr（交互）。
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
		p, err := r.Resolve(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		chosen = p
		extra = args[1:]
	}

	code2, err := launcher.New().Run(chosen, extra)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return code2
}

// buildRegistry 加载两来源并合并；两来源都空时给出引导。
func buildRegistry(cfg config.Config) (*registry.Registry, int) {
	profiles, errs := source.LoadAll(
		source.NewCCSwitch(cfg.DB),
		source.NewCustomDir(cfg.ProfilesDir),
	)
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, "警告:", e)
	}
	if len(profiles) == 0 {
		fmt.Fprintf(os.Stderr,
			"没有找到任何配置。\n未检测到 cc-switch，或没有自定义 profile。\n"+
				"可在 %s 下放一个 JSON，例如 deepseek.json：\n"+
				"  {\"model\":\"sonnet\",\"env\":{\"ANTHROPIC_BASE_URL\":\"...\",\"ANTHROPIC_AUTH_TOKEN\":\"...\"}}\n"+
				"或运行 `ccr edit deepseek` 直接创建。\n",
			cfg.ProfilesDir)
		return nil, 1
	}
	return registry.New(profiles), 0
}

// cmdLs 打印所有 profile（不泄露 token）。
func cmdLs(r *registry.Registry, out io.Writer) int {
	for _, p := range r.List() {
		cur := " "
		if p.IsCurrent {
			cur = "●"
		}
		fmt.Fprintf(out, "%s %-20s [%-9s] %-10s %s\n", cur, p.Name, p.Source, p.Model, p.BaseURL)
	}
	return 0
}

// runShow 解析 show 的参数后调用 cmdShow。
func runShow(cfg config.Config, args []string) int {
	reveal := false
	var name string
	for _, a := range args {
		if a == "--reveal" {
			reveal = true
		} else {
			name = a
		}
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, "用法: ccr show <名字> [--reveal]")
		return 1
	}
	r, code := buildRegistry(cfg)
	if code != 0 {
		return code
	}
	return cmdShow(r, name, reveal, os.Stdout)
}

// cmdShow 打印某 profile 的完整内容；reveal=false 时 token 打码。
func cmdShow(r *registry.Registry, name string, reveal bool, out io.Writer) int {
	p, err := r.Resolve(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	env := p.Env
	if !reveal {
		env = profile.RedactEnv(p.Env)
	}
	fmt.Fprintf(out, "名字:   %s\n来源:   %s\n模型:   %s\n", p.Name, p.Source, p.Model)
	fmt.Fprintln(out, "环境变量:")
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(out, "  %s=%s\n", k, env[k])
	}
	return 0
}

// runEdit 用 $EDITOR 打开/新建自定义 profile 的 JSON 文件。
func runEdit(cfg config.Config, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "用法: ccr edit <名字>")
		return 1
	}
	name := args[0]
	path := filepath.Join(cfg.ProfilesDir, name+".json")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(cfg.ProfilesDir, 0o700); err != nil {
			fmt.Fprintln(os.Stderr, "创建目录失败:", err)
			return 1
		}
		tmpl, _ := json.MarshalIndent(map[string]any{
			"model": "",
			"env": map[string]string{
				"ANTHROPIC_BASE_URL":   "",
				"ANTHROPIC_AUTH_TOKEN": "",
			},
		}, "", "  ")
		if err := os.WriteFile(path, tmpl, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, "写入模板失败:", err)
			return 1
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "vi"
		}
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "编辑器退出异常:", err)
		return 1
	}
	fmt.Println("已保存:", path)
	return 0
}

// printUsage 打印帮助。
func printUsage(out io.Writer) {
	fmt.Fprint(out, `ccr — 用选定 provider 的环境变量启动 claude

用法:
  ccr                      交互式选择一个配置并启动
  ccr <名字> [claude参数]   按名字直启，多余参数透传给 claude
  ccr ls                   列出所有配置（两来源）
  ccr show <名字> [--reveal] 查看某配置（默认 token 打码）
  ccr edit <名字>           用 $EDITOR 编辑/新建自定义配置

配置来源: cc-switch 库 + 自定义目录（~/.ccr/profiles/*.json）
`)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/cli/ -v`
Expected: PASS。

- [ ] **Step 5: 接入 main.go**

`main.go`:
```go
package main

import (
	"os"

	"ccr/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
```

- [ ] **Step 6: 全量测试 + 构建**

Run:
```
go test ./... ; go build -o ccr.exe .
```
Expected: 所有包 PASS；生成 `ccr.exe`。

- [ ] **Step 7: 提交**

```bash
git add internal/cli/ main.go
git commit -m "feat(cli): 参数分发与各层装配"
```

---

## Task 10: 端到端手动验证、README 与交叉编译

**Files:**
- Create: `README.md`

- [ ] **Step 1: 用真实 cc-switch 库手动验证 ls**

Run: `./ccr.exe ls`
Expected: 列出你 cc-switch 里的 claude provider（如 default、火山 Coding Plan、DeepSeek、Claude Official），带来源 `[cc-switch]`，当前项有 ●，**token 不出现**。

- [ ] **Step 2: 手动验证 show 打码 / reveal**

Run: `./ccr.exe show DeepSeek`
Expected: 打印环境变量，`ANTHROPIC_AUTH_TOKEN` 显示为 `sk-xxxx…` 打码形式。
Run: `./ccr.exe show DeepSeek --reveal`
Expected: 显示完整 token。

- [ ] **Step 3: 手动验证交互选择 + 启动**

Run: `./ccr.exe`
Expected: 弹出可上下选/可输入过滤的清单；选中后启动 `claude` 且使用该 provider（在 claude 里确认走对了后端/模型）。按 Esc 应打印"已取消"并退出码 1。

- [ ] **Step 4: 手动验证直启 + 参数透传**

Run: `./ccr.exe DeepSeek --version`（`--version` 透传给 claude）
Expected: claude 以 DeepSeek 配置启动并响应 `--version`。

- [ ] **Step 5: 手动验证自定义来源与 edit**

Run: `./ccr.exe edit my-test`
Expected: 在 `~/.ccr/profiles/my-test.json` 生成模板并用编辑器打开；填好 `ANTHROPIC_BASE_URL`/`ANTHROPIC_AUTH_TOKEN` 保存后，`./ccr.exe ls` 出现 `my-test [custom]`。

- [ ] **Step 6: 写 README**

`README.md`：
```markdown
# ccr

用选定 provider 的环境变量启动 `claude`，从而可同时多开、各用不同后端，互不干扰，且不改全局配置。

## 安装
```
go build -o ccr .   # 或交叉编译，见下
```
把生成的二进制放进 PATH。

## 用法
- `ccr` 交互式选择并启动
- `ccr <名字> [claude 参数]` 直启并透传参数
- `ccr ls` 列出全部配置
- `ccr show <名字> [--reveal]` 查看配置（默认打码）
- `ccr edit <名字>` 编辑/新建自定义配置

## 配置来源
1. cc-switch 库（默认 `~/.cc-switch/cc-switch.db`，只读）
2. 自定义目录 `~/.ccr/profiles/*.json`，每个文件一个配置：
   ```json
   {"model":"sonnet","env":{"ANTHROPIC_BASE_URL":"...","ANTHROPIC_AUTH_TOKEN":"..."}}
   ```
覆盖路径：`CCR_DB` / `CCR_PROFILES_DIR` 环境变量，或 `~/.ccr/config.json`。
```

- [ ] **Step 7: 交叉编译三平台**

Run（PowerShell）:
```
$env:CGO_ENABLED=0
foreach ($t in @("windows/amd64","darwin/arm64","darwin/amd64","linux/amd64")) {
  $p=$t.Split("/"); $env:GOOS=$p[0]; $env:GOARCH=$p[1]
  $ext=if($p[0] -eq "windows"){".exe"}else{""}
  go build -o "dist/ccr-$($p[0])-$($p[1])$ext" .
}
Remove-Item Env:GOOS,Env:GOARCH
```
Expected: `dist/` 下生成四个二进制，命令成功无报错。

- [ ] **Step 8: 提交**

```bash
git add README.md
git commit -m "docs: README 与交叉编译说明"
```

---

## Self-Review（规格覆盖核对）

- 规格 §4 按会话注入 env → Task 7 `ComposeEnv` + `Run`。✓
- §5 命令面 `ls`/`show`/`edit`/直启/交互 → Task 9。✓
- §6.1 Profile 模型 → Task 1。✓
- §6.2 两来源 → Task 3（custom）、Task 4（ccswitch）。✓
- §6.3 合并/重名消歧/来源标记 → Task 6。✓
- §7 数据流（model→--model、env 覆盖）→ Task 7 `ClaudeArgs`/`ComposeEnv`。✓
- §8 路径解析 env>文件>默认 → Task 5。✓
- §9 cc-switch 只读访问 → Task 4（`mode=ro` + `busy_timeout`）。✓
- §10 自定义目录 JSON 同构 → Task 3 + Task 9 `edit` 模板。✓
- §11 跨平台拉起、透传退出码 → Task 7。✓
- §12 错误处理（未装/坏 JSON/未命中/找不到 claude）→ Task 3、4、6、7、9。✓
- §13 安全（token 打码、0600）→ Task 1、9。✓
- §14 测试（真实样本/临时 fixture/helper-process）→ Task 2、3、4、7。✓
- §15 交叉编译 → Task 10。✓

类型一致性：`Profile`、`Source`、`ParseSettingsConfig`、`ComposeEnv`、`ClaudeArgs`、`Launcher.Run`、`cmdLs`/`cmdShow` 在定义与引用处签名一致。✓

> 已知简化（符合 §3 YAGNI）：§9 的"库被锁→拷临时文件"兜底未单列任务——`mode=ro` 已能与运行中的 cc-switch 并发读，拷贝兜底留待真出现锁问题时再加。信号转发依赖终端把信号投递给前台子进程（继承 stdio），未额外处理。
