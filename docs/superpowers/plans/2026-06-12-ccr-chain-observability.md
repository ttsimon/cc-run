# ccr chain 执行可观测性与输出样式（track B）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
> **提交约定**：Conventional Commits；每条 commit 信息末尾加 `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`。提交前过 `task check`。
> **前置依赖**：track A（隔离）已合并——放行点的 `git diff --stat` 复用其 worktree。若 A 未完成，Task 5 的 diff 部分退化为对当前目录跑 diff，不阻塞其余部分。

**Goal:** 让 chain 执行看得见过程（逐工具调用 + 最终结果，详细度 `-q`/默认/`-v` 可调），并建一层共享终端样式，非 TTY 自动降级。

**Architecture:** 段一律以 `--output-format stream-json --verbose` 跑，ccr 解析这一条事件流；详细度只是渲染过滤器。`stream.go` 防脆解析事件，`render.go` 按级别 + TTY 渲染，runner 实时喂渲染器并从「最终 result 事件」抽取交棒文本。`ui/style.go` 提供共享 lipgloss 调色板/符号，chain 先消费。

**Tech Stack:** Go；`charmbracelet/lipgloss`（提为直接依赖）、`mattn/go-isatty`（已在树）、标准库 `encoding/json`/`bufio`。spec 见 `docs/superpowers/specs/2026-06-11-ccr-chain-observability-design.md`。

---

## File Structure

- **Create** `internal/ui/style.go` — 共享样式层：调色板、符号集、`IsTTY`/`WriterIsTTY`/`Apply` 助手。
- **Create** `internal/ui/style_test.go` — 样式层单测。
- **Create** `internal/chain/stream.go` — `Event`/`EventKind` + `ParseEventLine`（防脆）。
- **Create** `internal/chain/stream_test.go` — 解析单测。
- **Create** `internal/chain/render.go` — `Level` + `Renderer`（按级别 + TTY 渲染）。
- **Create** `internal/chain/render_test.go` — 渲染单测。
- **Modify** `internal/chain/runner.go` — `SegmentArgs` 改 stream-json；`RunSegment(spec, *Renderer)` 流式读取 + 抽 result + 防脆回退。
- **Modify** `internal/chain/runner_test.go` — 适配新签名与新旗标断言。
- **Modify** `internal/chain/orchestrate.go` — 加 `Level`；段框 + 计时；renderer 接线；放行点加厚（diff/耗时）。
- **Modify** `internal/chain/orchestrate_test.go` — 放行点加厚断言。
- **Modify** `internal/cli/cli.go` — 解析 `-q`/`-v`、互斥校验、help。
- **Modify** `internal/cli/cli_test.go` — `-q -v` 互斥测试。

---

## Task 1: 共享样式层 internal/ui/style.go

**Files:**
- Create: `internal/ui/style.go`
- Create: `internal/ui/style_test.go`

- [ ] **Step 1: 写失败测试 — Apply 在非 TTY 不染色、WriterIsTTY 对非文件返回 false**

`internal/ui/style_test.go`：

```go
package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestApply_非TTY不加ANSI(t *testing.T) {
	got := Apply(false, StyleOK, "ok")
	if got != "ok" {
		t.Errorf("非 TTY 应原样返回, got %q", got)
	}
	if strings.Contains(got, "\x1b[") {
		t.Errorf("非 TTY 不应含 ANSI: %q", got)
	}
}

func TestWriterIsTTY_非文件为false(t *testing.T) {
	if WriterIsTTY(&bytes.Buffer{}) {
		t.Error("bytes.Buffer 不是 TTY")
	}
}

func TestIcons_非空(t *testing.T) {
	for _, s := range []string{IconTool, IconOK, IconRun, IconPause} {
		if s == "" {
			t.Error("符号不应为空")
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/ui/ -v`
Expected: 失败（包 `ui` 不存在）。

- [ ] **Step 3: 创建 ui/style.go**

```go
// Package ui 提供 ccr 各命令共享的终端样式：调色板、符号集与 TTY 降级助手。
package ui

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

// 符号集（段框/工具行/状态）。
const (
	IconTool  = "🔧"
	IconOK    = "✔"
	IconRun   = "▶"
	IconPause = "⏸"
)

// 调色板（仅在 TTY 时经 Apply 生效）。
var (
	StyleSegment = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	StyleOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	StyleErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	StyleDim     = lipgloss.NewStyle().Faint(true)
)

// Apply 仅在 tty 为真时给 s 套样式，否则原样返回（非 TTY/管道/CI 降级）。
func Apply(tty bool, st lipgloss.Style, s string) string {
	if !tty {
		return s
	}
	return st.Render(s)
}

// IsTTY 报告 f 是否连到终端。
func IsTTY(f *os.File) bool {
	return isatty.IsTerminal(f.Fd())
}

// WriterIsTTY 在 w 是 *os.File 且为终端时返回 true；否则 false（含 bytes.Buffer 等）。
func WriterIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && IsTTY(f)
}
```

- [ ] **Step 4: 提为直接依赖并通过测试**

Run: `go mod tidy && go test ./internal/ui/ -v`
Expected: `lipgloss`/`go-isatty` 从 `// indirect` 升为直接依赖；测试 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/ui/style.go internal/ui/style_test.go go.mod go.sum
git commit -m "feat: shared ui style layer with tty downgrade"
```

---

## Task 2: stream.go 事件解析（防脆）

**Files:**
- Create: `internal/chain/stream.go`
- Create: `internal/chain/stream_test.go`

- [ ] **Step 1: 写失败测试 — 解析 tool_use/text/result、坏行/未知优雅降级、多块**

`internal/chain/stream_test.go`：

```go
package chain

import "testing"

func TestParseEventLine_工具调用(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"web/index.html"}}]}}`)
	evs := ParseEventLine(line)
	if len(evs) != 1 || evs[0].Kind != EventToolUse {
		t.Fatalf("应解析出一个工具事件, got %+v", evs)
	}
	if evs[0].Tool != "Write" || evs[0].Target != "web/index.html" {
		t.Errorf("工具名/目标错: %+v", evs[0])
	}
}

func TestParseEventLine_Bash取command(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"git add -A\ngit commit"}}]}}`)
	evs := ParseEventLine(line)
	if len(evs) != 1 || evs[0].Target != "git add -A …" {
		t.Errorf("Bash 应取首行 command + 省略号: %+v", evs)
	}
}

func TestParseEventLine_result(t *testing.T) {
	line := []byte(`{"type":"result","subtype":"success","result":"最终答案","usage":{"input_tokens":10,"output_tokens":5}}`)
	evs := ParseEventLine(line)
	if len(evs) != 1 || evs[0].Kind != EventResult {
		t.Fatalf("应解析出 result 事件, got %+v", evs)
	}
	if evs[0].Text != "最终答案" {
		t.Errorf("result 文本错: %q", evs[0].Text)
	}
	if evs[0].Usage == "" {
		t.Errorf("result 应带 usage 摘要")
	}
}

func TestParseEventLine_坏行与未知优雅降级(t *testing.T) {
	if evs := ParseEventLine([]byte(`{不是合法json`)); evs != nil {
		t.Errorf("坏 JSON 应返回 nil, got %+v", evs)
	}
	if evs := ParseEventLine([]byte(`{"type":"system","subtype":"init"}`)); evs != nil {
		t.Errorf("未知/无关 type 应返回 nil, got %+v", evs)
	}
	if evs := ParseEventLine([]byte("   ")); evs != nil {
		t.Errorf("空行应返回 nil")
	}
}

func TestParseEventLine_一条消息多块(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"我来写文件"},{"type":"tool_use","name":"Read","input":{"file_path":"a.go"}}]}}`)
	evs := ParseEventLine(line)
	if len(evs) != 2 {
		t.Fatalf("一条消息两块应出两事件, got %d", len(evs))
	}
	if evs[0].Kind != EventAssistantText || evs[1].Kind != EventToolUse {
		t.Errorf("块顺序/类型错: %+v", evs)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/chain/ -run TestParseEventLine -v`
Expected: 失败（`Event`/`ParseEventLine` 未定义）。

- [ ] **Step 3: 创建 stream.go**

```go
package chain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// EventKind 是从 stream-json 抽出的内部事件类型（与具体 schema 解耦）。
type EventKind int

const (
	EventOther EventKind = iota
	EventToolUse
	EventAssistantText
	EventResult
)

// Event 是段无关的渲染单元。
type Event struct {
	Kind   EventKind
	Tool   string // EventToolUse：工具名
	Target string // EventToolUse：主要参数摘要（文件路径/命令首行）
	Text   string // EventAssistantText / EventResult：文本
	Usage  string // EventResult：token 摘要
}

// rawEvent 对应 claude -p --output-format stream-json --verbose 的一行。
//
// ⚠️ 集成边界：字段名按当前 Claude Code stream-json 写。实现时跑一次真
// `claude -p --output-format stream-json --verbose` 校准；schema 漂移只改本文件。
type rawEvent struct {
	Type    string `json:"type"`
	Result  string `json:"result"`
	Message struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	} `json:"message"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ParseEventLine 把一行 stream-json 解析为零或多个内部事件。
// 防脆：空行/坏 JSON/未知 type 一律返回 nil，绝不报错中断整段。
func ParseEventLine(line []byte) []Event {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return nil
	}
	var raw rawEvent
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil
	}
	switch raw.Type {
	case "assistant":
		var evs []Event
		for _, b := range raw.Message.Content {
			switch b.Type {
			case "text":
				if strings.TrimSpace(b.Text) != "" {
					evs = append(evs, Event{Kind: EventAssistantText, Text: b.Text})
				}
			case "tool_use":
				evs = append(evs, Event{
					Kind:   EventToolUse,
					Tool:   b.Name,
					Target: toolTarget(b.Input),
				})
			}
		}
		return evs
	case "result":
		return []Event{{
			Kind:  EventResult,
			Text:  raw.Result,
			Usage: fmt.Sprintf("in %d / out %d", raw.Usage.InputTokens, raw.Usage.OutputTokens),
		}}
	default:
		return nil // system / user(tool_result) / 未知 → 不渲染
	}
}

// toolTarget 从工具 input 抽一个简短目标；抽不到返回空。
func toolTarget(input json.RawMessage) string {
	var m map[string]any
	if json.Unmarshal(input, &m) != nil {
		return ""
	}
	for _, k := range []string{"file_path", "path", "command", "pattern", "url"} {
		if v, ok := m[k].(string); ok && v != "" {
			return firstLine(v)
		}
	}
	return ""
}

// firstLine 取首行；有多行时加省略标记。
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i]) + " …"
	}
	return s
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/chain/ -run TestParseEventLine -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/chain/stream.go internal/chain/stream_test.go
git commit -m "feat: brittle-resistant stream-json event parser for chain segments"
```

---

## Task 3: render.go 按级别渲染

**Files:**
- Create: `internal/chain/render.go`
- Create: `internal/chain/render_test.go`

- [ ] **Step 1: 写失败测试 — quiet/normal/verbose 渲染对应子集**

`internal/chain/render_test.go`：

```go
package chain

import (
	"strings"
	"testing"
)

func renderAll(level Level, evs ...Event) string {
	var b strings.Builder
	r := &Renderer{Level: level, TTY: false, Out: &b}
	for _, e := range evs {
		r.Render(e)
	}
	return b.String()
}

func TestRenderer_quiet不打工具行(t *testing.T) {
	out := renderAll(LevelQuiet,
		Event{Kind: EventToolUse, Tool: "Write", Target: "a.html"},
		Event{Kind: EventAssistantText, Text: "思考"},
	)
	if strings.Contains(out, "Write") || strings.Contains(out, "思考") {
		t.Errorf("quiet 不应打工具/思考行: %q", out)
	}
}

func TestRenderer_normal打工具行不打思考(t *testing.T) {
	out := renderAll(LevelNormal,
		Event{Kind: EventToolUse, Tool: "Write", Target: "a.html"},
		Event{Kind: EventAssistantText, Text: "思考内容"},
	)
	if !strings.Contains(out, "Write") || !strings.Contains(out, "a.html") {
		t.Errorf("normal 应打工具行: %q", out)
	}
	if strings.Contains(out, "思考内容") {
		t.Errorf("normal 不应打思考行: %q", out)
	}
}

func TestRenderer_verbose打思考与token(t *testing.T) {
	out := renderAll(LevelVerbose,
		Event{Kind: EventAssistantText, Text: "思考内容"},
		Event{Kind: EventResult, Text: "答案", Usage: "in 10 / out 5"},
	)
	if !strings.Contains(out, "思考内容") {
		t.Errorf("verbose 应打思考行: %q", out)
	}
	if !strings.Contains(out, "in 10 / out 5") {
		t.Errorf("verbose 应打 token: %q", out)
	}
}

func TestRenderer_非TTY无ANSI(t *testing.T) {
	out := renderAll(LevelNormal, Event{Kind: EventToolUse, Tool: "Bash", Target: "ls"})
	if strings.Contains(out, "\x1b[") {
		t.Errorf("非 TTY 不应含 ANSI: %q", out)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/chain/ -run TestRenderer -v`
Expected: 失败（`Level`/`Renderer` 未定义）。

- [ ] **Step 3: 创建 render.go**

```go
package chain

import (
	"fmt"
	"io"
	"strings"

	"github.com/ttsimon/cc-run/internal/ui"
)

// Level 是运行时详细度。
type Level int

const (
	LevelQuiet   Level = iota // -q：仅段框 + 最终结果
	LevelNormal               // 默认：+ 逐工具调用
	LevelVerbose              // -v：+ 思考文本 + token
)

// Renderer 按级别与 TTY 把事件渲染到 Out。result 文本不在此打印——
// 由 orchestrator 在段尾决定如何展示（交棒/段框）。
type Renderer struct {
	Level Level
	TTY   bool
	Out   io.Writer
}

func (r *Renderer) Render(e Event) {
	if r == nil || r.Out == nil {
		return
	}
	switch e.Kind {
	case EventToolUse:
		if r.Level >= LevelNormal {
			line := fmt.Sprintf("  %s %s %s", ui.IconTool, e.Tool, e.Target)
			fmt.Fprintln(r.Out, ui.Apply(r.TTY, ui.StyleDim, strings.TrimRight(line, " ")))
		}
	case EventAssistantText:
		if r.Level >= LevelVerbose {
			fmt.Fprintln(r.Out, ui.Apply(r.TTY, ui.StyleDim, "  "+strings.TrimSpace(e.Text)))
		}
	case EventResult:
		if r.Level >= LevelVerbose && e.Usage != "" {
			fmt.Fprintln(r.Out, ui.Apply(r.TTY, ui.StyleDim, "  [tokens] "+e.Usage))
		}
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/chain/ -run TestRenderer -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/chain/render.go internal/chain/render_test.go
git commit -m "feat: level- and tty-aware renderer for chain stream events"
```

---

## Task 4: runner 改流式（stream-json + 抽取 result）

`SegmentArgs` 切到 stream-json；`RunSegment` 加渲染器参数、流式读取、从 result 事件抽交棒文本，无 result 则回退整段 stdout（防脆）。

**Files:**
- Modify: `internal/chain/runner.go`
- Modify: `internal/chain/runner_test.go`

- [ ] **Step 1: 改测试断言新旗标 + 新签名**

在 `internal/chain/runner_test.go`：

把 `TestSegmentArgs_基本` 的期望从 `"--output-format", "text"` 改为同时含 `"--output-format", "stream-json"` 与 `"--verbose"`：

```go
for _, want := range []string{"-p", "照做", "--allowedTools", "Read,Write", "--add-dir", "/tmp/wd", "--output-format", "stream-json", "--verbose"} {
```

把两处 `r.RunSegment(runSpec{...})` 调用改为传 `nil` 渲染器：`r.RunSegment(runSpec{...}, nil)`（两个测试 `TestRunSegment_在Workdir里运行`、`TestRunSegment_捕获stdout并注入env` 各一处）。

> helper-process 在 stdout 打的是纯文本（非 JSON）→ `ParseEventLine` 返回 nil → 无 result 事件 → 回退整段 stdout，故这两个测试的输出断言不变。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/chain/ -run 'TestSegmentArgs_基本|TestRunSegment_' -v`
Expected: 失败（旗标不匹配 / `RunSegment` 参数个数不符）。

- [ ] **Step 3: 改 runner.go**

把 `SegmentArgs` 的 args 头改为：

```go
	args := []string{"-p", prompt, "--output-format", "stream-json", "--verbose", "--add-dir", workdir}
```

把 `RunSegment` 整体替换为（新增 import：`bufio`；移除不再用的 `bytes`）：

```go
// RunSegment 跑一段：流式解析 claude 的 stream-json，逐事件喂给 rnd 渲染，
// 返回 (最终 result 文本, 退出码, error)。rnd 为 nil 时不渲染、仅抽取结果。
// 无 result 事件时回退为整段 stdout（防脆）。
func (r *Runner) RunSegment(spec runSpec, rnd *Renderer) (string, int, error) {
	path := r.ClaudePath
	if path == "" {
		found, err := r.LookPath("claude")
		if err != nil {
			return "", -1, fmt.Errorf("找不到 claude 可执行文件：%w", err)
		}
		path = found
	}

	args := append(spec.ExtraArgs, SegmentArgs(spec.Prompt, spec.AllowTools, spec.Workdir, spec.SettingsPath)...)

	cmd := exec.Command(path, args...)
	cmd.Dir = spec.Workdir
	cmd.Env = launcher.ComposeEnv(r.Environ(), spec.Env)
	cmd.Stderr = r.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", -1, fmt.Errorf("接管 stdout 失败: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", -1, fmt.Errorf("启动 claude 失败: %w", err)
	}

	var resultText string
	var rawAll strings.Builder
	reader := bufio.NewReader(stdout)
	for {
		line, rerr := reader.ReadString('\n')
		if len(line) > 0 {
			rawAll.WriteString(line)
			for _, e := range ParseEventLine([]byte(line)) {
				rnd.Render(e) // Renderer.Render 对 nil 接收者安全
				if e.Kind == EventResult {
					resultText = e.Text
				}
			}
		}
		if rerr != nil {
			break
		}
	}

	out := resultText
	if out == "" {
		out = strings.TrimSpace(rawAll.String()) // 防脆回退：无 result 事件
	}

	werr := cmd.Wait()
	if werr == nil {
		return out, 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(werr, &exitErr) {
		return out, exitErr.ExitCode(), nil
	}
	return out, -1, werr
}
```

> `Renderer.Render` 已对 `nil` 接收者做了短路（见 Task 3），故 `rnd.Render(e)` 在 `rnd==nil` 时安全。

更新 `runner.go` 顶部 import：去掉 `"bytes"`，加 `"bufio"`。`io`/`os` 仍由 Runner 字段使用，保留。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/chain/ -run 'TestSegmentArgs|TestRunSegment_' -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/chain/runner.go internal/chain/runner_test.go
git commit -m "feat: stream-json segment run, render live and extract result handoff"
```

---

## Task 5: orchestrate 接线 — 段框/计时/渲染器/放行点加厚

**Files:**
- Modify: `internal/chain/orchestrate.go`
- Modify: `internal/chain/orchestrate_test.go`

- [ ] **Step 1: 写失败测试 — 放行点加厚含耗时；段框打印；Level 字段存在**

追加到 `internal/chain/orchestrate_test.go`：

```go
func TestOrchestrate_放行点加厚含耗时(t *testing.T) {
	c := Chain{Workdir: t.TempDir(), Segments: []Segment{
		{Name: "a", Profile: "strong", Prompt: "x"},
		{Name: "b", Profile: "cheap", Prompt: "y"},
	}}
	cp := &capturePauser{d: DecisionProceed}
	o := NewOrchestrator(testReg())
	o.Auto = false
	o.Pauser = cp
	o.Out = &strings.Builder{}
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { return "o", 0, nil }
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cp.got, "耗时") {
		t.Errorf("放行点应含耗时: %q", cp.got)
	}
}

func TestOrchestrate_打印段框(t *testing.T) {
	var out strings.Builder
	c := Chain{Workdir: t.TempDir(), Segments: []Segment{{Name: "impl", Profile: "strong", Prompt: "x"}}}
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.Out = &out
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { return "o", 0, nil }
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "impl") || !strings.Contains(s, "1/1") {
		t.Errorf("应打印段框（名+进度）: %q", s)
	}
}

func TestOrchestrate_Level默认normal(t *testing.T) {
	o := NewOrchestrator(testReg())
	if o.Level != LevelNormal {
		t.Errorf("默认级别应为 normal, got %v", o.Level)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/chain/ -run 'TestOrchestrate_放行点加厚|TestOrchestrate_打印段框|TestOrchestrate_Level' -v`
Expected: 失败（`o.Level`/段框/耗时未实现）。

- [ ] **Step 3: 改 orchestrate.go**

import 加 `"time"` 和 `"github.com/ttsimon/cc-run/internal/ui"`。给结构体加字段：

```go
	Out    io.Writer
	Level  Level // 渲染详细度；默认 LevelNormal
```

在 `NewOrchestrator`：默认级别（零值即 `LevelNormal`，无需显式赋值，但默认 runSegment 闭包需接新签名并注入渲染器）。把闭包改为：

```go
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		out := o.Out
		if out == nil {
			out = os.Stdout
		}
		rnd := &Renderer{Level: o.Level, TTY: ui.WriterIsTTY(out), Out: out}
		return runner.RunSegment(spec, rnd)
	}
```

在 `Run` 的 for 循环里，给段加框 + 计时。把跑段那段（`spec := runSpec{...}` 到 `prev = segOut` 之间）包成：

```go
		fmt.Fprintf(out, "%s 段 %d/%d %s [%s]\n",
			ui.Apply(ui.WriterIsTTY(out), ui.StyleSegment, ui.IconRun),
			i+1, len(c.Segments), seg.Name, seg.Profile)

		start := time.Now()
		spec := runSpec{
			Prompt:       renderedPrompt,
			AllowTools:   seg.AllowTools,
			Workdir:      workdir,
			SettingsPath: settingsPath,
			Env:          env,
		}
		segOut, code, err := o.runSegment(spec, seg)
		// ... 既有 err / code != 0 处理保持不变（track A 已加 abandon）...
		prev = segOut
		elapsed := time.Since(start).Round(time.Second)
		fmt.Fprintf(out, "%s 段 %d/%d 完成 (%s)\n",
			ui.Apply(ui.WriterIsTTY(out), ui.StyleOK, ui.IconOK),
			i+1, len(c.Segments), elapsed)
		if o.Auto && strings.TrimSpace(prev) != "" {
			fmt.Fprintln(out, prev) // auto 无放行点，结果在此回显
		}
```

> 注：若 track A 已落地，`err`/`code != 0` 分支里已含 `abandon(out, iso)`；本任务不改那两行，只在其前打段框、其后打 footer。`elapsed` 需在放行点用到，故声明在循环作用域内（见下）。

放行点加厚：把 `info` 组装改为追加 diff 与耗时。在构造 `info` 处之后、调用 `o.Pauser.Pause` 之前插入：

```go
			if ds := segmentDiffStat(workdir); ds != "" {
				info += "\n本段改动:\n" + ds
			}
			info += fmt.Sprintf("\n耗时 %s", elapsed)
```

> `elapsed` 在本段 footer 处已算出；确保其声明在 for 循环体顶层作用域（用 `elapsed := time.Since(start)...`，放行点在同一次迭代内可见）。

在文件末尾新增 best-effort diff 助手：

```go
// segmentDiffStat 返回隔离 worktree 里本段相对上一提交的 diff --stat；
// 非 git / 无上一提交 / 出错时返回 ""（best-effort，不影响主流程）。
func segmentDiffStat(workdir string) string {
	out, err := gitIn(workdir, "diff", "--stat", "HEAD~1", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
```

> `gitIn` 来自 track A 的 `isolate.go`。若 track A 尚未合并，临时在本文件加一份同名助手（合并 A 后去重）。

- [ ] **Step 4: 运行 chain 全包回归**

Run: `go test ./internal/chain/ -v`
Expected: 全 PASS（既有 orchestrate 测试中 `runSegment` 被注入，不走真实 runner，段框/耗时打印不破坏既有断言；放行点既有断言 `Contains("needs-work")` 仍成立）。

- [ ] **Step 5: 提交**

```bash
git add internal/chain/orchestrate.go internal/chain/orchestrate_test.go
git commit -m "feat: segment frames, timing, renderer wiring and richer pause point"
```

---

## Task 6: CLI 旗标 -q/-v + 互斥校验

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: 写失败测试 — -q -v 同给报错**

追加到 `internal/cli/cli_test.go`：

```go
func TestRunChain_quiet和verbose互斥(t *testing.T) {
	f := writeChainFile(t, chainNoInput)
	var code int
	errOut := captureStderr(t, func() { code = Execute([]string{"chain", f, "-q", "-v"}) })
	if code == 0 {
		t.Error("同时给 -q -v 应非 0")
	}
	if !strings.Contains(errOut, "-q") && !strings.Contains(errOut, "互斥") {
		t.Errorf("应提示 -q/-v 互斥, stderr=%q", errOut)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/cli/ -run TestRunChain_quiet和verbose互斥 -v`
Expected: 失败（旗标未解析，`-q`/`-v` 被当成文件名 → 可能以其它错误失败或 code 0）。

- [ ] **Step 3: 改 cli.go runChain**

在 `runChain` 的旗标解析循环里，`auto`/`input` 声明旁加 `quiet`/`verbose`：

```go
	auto := false
	quiet := false
	verbose := false
	inputProvided := false
	var input, file string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--auto":
			auto = true
		case "-q", "--quiet":
			quiet = true
		case "-v", "--verbose":
			verbose = true
		case "--input", "-i":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--input 后面要跟需求文本")
				return 1
			}
			i++
			input, inputProvided = args[i], true
		default:
			file = args[i]
		}
	}
	if quiet && verbose {
		fmt.Fprintln(os.Stderr, "-q 与 -v 互斥，只能选一个")
		return 1
	}
```

在 `o := chain.NewOrchestrator(r)` 之后、`o.Run(c)` 之前设级别：

```go
	o.Auto = auto
	o.Input = input
	switch {
	case quiet:
		o.Level = chain.LevelQuiet
	case verbose:
		o.Level = chain.LevelVerbose
	default:
		o.Level = chain.LevelNormal
	}
```

更新用法串（两处 `用法:` 与 `printUsage`）补 `[-q | -v]`：

```go
	fmt.Fprintln(os.Stderr, `用法: ccr chain <chain.yaml> [--auto] [--input "需求"] [-q | -v]`)
```

`printUsage` 里 chain 行改为：

```go
  ccr chain <file> [--auto] [--input "需求"] [-q|-v]
                               跑一条多后端流水线（-q 静默/-v 详细）；ccr chain init 生成模板
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/cli/ -run TestRunChain -v`
Expected: PASS（互斥用例 + 既有 chain 用例）。

- [ ] **Step 5: 提交**

```bash
git add internal/cli/cli.go internal/cli/cli_test.go
git commit -m "feat: ccr chain -q/-v verbosity flags with mutual-exclusion check"
```

---

## Task 7: 校准 stream-json + 文档 + 全套校验

**Files:**
- Modify: `internal/chain/stream.go`（仅在校准发现 schema 差异时）
- Modify: `docs/superpowers/specs/2026-06-09-ccr-chain-design.md`、`CLAUDE.md`

- [ ] **Step 1: 用真 claude 校准事件结构**

Run（需本机装了 claude；无则跳过并在 commit 说明里注明未校准）：
```bash
claude -p "列出当前目录文件" --output-format stream-json --verbose
```
Expected: 逐行 JSON。核对 `type`/`message.content[].type`/`tool_use.name`/`tool_use.input`/`result`/`usage.input_tokens`/`usage.output_tokens` 字段名与 `stream.go` 的 `rawEvent` 一致。**若有差异，只改 `internal/chain/stream.go` 的 `rawEvent` 标签**，并据需调整 `stream_test.go` 样例。

- [ ] **Step 2: 若校准有改动，跑解析测试**

Run: `go test ./internal/chain/ -run TestParseEventLine -v`
Expected: PASS。

- [ ] **Step 3: 文档同步**

在 `docs/superpowers/specs/2026-06-09-ccr-chain-design.md` 补一行：

```markdown
- **可观测性**：段以 stream-json 跑，ccr 实时渲染工具调用与结果，详细度 `-q`/默认/`-v` 可调；非 TTY 自动降级。共享样式层 `internal/ui`。详见 specs/2026-06-11-ccr-chain-observability-design.md。
```

在 `CLAUDE.md` 代码布局表加一行：

```markdown
internal/ui/         共享终端样式层（lipgloss 调色板/符号 + TTY 降级）
```

- [ ] **Step 4: 全套校验**

Run: `task check`
Expected: fmt/vet/lint/test 全绿。

- [ ] **Step 5: 提交**

```bash
git add internal/chain/stream.go internal/chain/stream_test.go docs/superpowers/specs/2026-06-09-ccr-chain-design.md CLAUDE.md
git commit -m "docs: document chain observability and ui style layer; calibrate stream-json"
```

---

## Self-Review

**Spec coverage（对照 observability-design.md）：**
- 段一律 stream-json + `--verbose` → Task 4（`SegmentArgs`）。✓
- eventParser（防脆，未知/坏行降级，标出最终 result）→ Task 2。✓
- renderer（quiet/normal/verbose 三级 + 段框由 orchestrate 打）→ Task 3 + Task 5。✓
- 交棒抽取改自 result 事件，无则回退 stdout → Task 4。✓
- 放行点加厚（diff --stat / verdict / 耗时）→ Task 5（verdict 既有；diff/耗时新增）。✓
- 样式层 `internal/ui/style.go`（lipgloss + isatty + NO_COLOR/非 TTY 降级）→ Task 1。✓
- CLI `-q`/`-v`，与 `--auto`/`--input` 正交，`-q -v` 互斥报错 → Task 6。✓
- 非 TTY 降级（无颜色/无转圈）→ Task 1 `Apply`、Task 3 `TTY:false`、Task 5 `WriterIsTTY`。✓
- stream-json 集成边界校准 → Task 7。✓
- 文档 → Task 7。✓
- **非目标**（详细度做成 yaml 字段、一次性重排所有命令、精确计费）均未做，符合 YAGNI。✓

**说明/取舍：** 转圈动画（spinner）spec 列为「运行中转圈 + 计时（仅 TTY）」——本计划以「段框 header/footer + 耗时」覆盖计时，转圈未实现（headless 段是阻塞调用，转圈需额外 goroutine，收益低，YAGNI 暂略）。若需要，可后续在 Task 5 段框 header 后起一个 TTY-only spinner goroutine、段尾停。已在此标注，不留隐式缺口。

**Placeholder scan：** 无 TBD/“适当处理”。Task 5 对 track A 未合并情形给了明确退路（临时同名 `gitIn`/diff 退化），非占位。

**Type consistency：** `Event{Kind,Tool,Target,Text,Usage}` 与 `EventKind` 常量在 Task 2 定义，Task 3 渲染、Task 4 抽取一致使用；`Level`(`LevelQuiet/Normal/Verbose`) Task 3 定义，Task 5/6 使用一致；`Renderer{Level,TTY,Out}` Task 3 定义，Task 4 构造、Task 5 注入一致；`RunSegment(spec, *Renderer)` 新签名 Task 4 定义，Task 5 闭包与 runner_test 调用一致；`ui.Apply/WriterIsTTY/IsTTY/Icon*/Style*` Task 1 定义，Task 3/5 使用一致。
