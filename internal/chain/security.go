package chain

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultDenylist 是 ccr 内置的命令红线（子串匹配，命中即拦）。
func DefaultDenylist() []string {
	return []string{
		"rm -rf",
		"git push",
		"shutdown",
		"mkfs",
		":(){ :|:& };:", // fork bomb
		"dd if=",
		"> /dev/sd",
	}
}

// MergeDenylist 合并默认与用户追加。
func MergeDenylist(base, extra []string) []string {
	return append(append([]string{}, base...), extra...)
}

// Denied 报告 cmd 是否命中黑名单（任一项为其子串，大小写不敏感）。
// 大小写不敏感是必要的：Windows PowerShell 不区分大小写，RM/Remove-Item 等
// 大写变体否则可绕过红线。
func Denied(cmd string, denylist []string) bool {
	lc := strings.ToLower(cmd)
	for _, d := range denylist {
		if d != "" && strings.Contains(lc, strings.ToLower(d)) {
			return true
		}
	}
	return false
}

// cdEscapeRE 匹配明显跳出当前工作目录的 cd：cd .. / cd / / cd ~ / cd $HOME。
// 命令前界用行首或 ; & | 空白；故意不拦 cd ./subdir、cd subdir/foo——这些不离开 workdir。
// 注：cd subdir/.. 这种"先进后退"的反模式会被拦——agent 几乎不会这么写，宁可误伤。
var cdEscapeRE = regexp.MustCompile(`(?i)(^|[\s;&|])cd\s+(\.\.|/|~|\$home)`)

// BashEscapesWorkdir 检测 Bash/PowerShell 命令里"cd 上跳"。
// shell 解析无穷变体，这里只挡最明显的姿势；越界写文件靠路径围栏，越界副作用靠
// 工具白名单 + isolate 隔离区兜底。
func BashEscapesWorkdir(cmd string) bool {
	if cmd == "" {
		return false
	}
	return cdEscapeRE.MatchString(cmd)
}

// PathEscapes 报告 rawPath 是否落在 workdir 与 allowed 白名单之外。
// 相对路径基于 workdir resolve；含 .. 经 filepath.Clean 展开后再判前缀。
// workdir 为空（无围栏 env）时一律放行——不阻断本就不该被钩子管的环境。
//
// 路径用 canonPath 解析软链：orchestrator 注入的 CCR_CHAIN_WORKDIR 已经是 realpath
// 形式（macOS 把 /var/folders 解到 /private/var/folders），agent 给的字面路径也得
// 同向解析才能正确判前缀，否则 /var/x 与 /private/var/x 会被当成两条不同路径。
func PathEscapes(rawPath, workdir string, allowed []string) bool {
	if rawPath == "" || workdir == "" {
		return false
	}
	abs := rawPath
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(workdir, abs)
	}
	abs = canonPath(abs)
	if pathContains(canonPath(workdir), abs) {
		return false
	}
	for _, a := range allowed {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if pathContains(canonPath(a), abs) {
			return false
		}
	}
	return true
}

// pathContains 判断 child 是否在 parent 内（或就是 parent）。
// 末尾加分隔符避免 /foo 误匹 /foobar。
func pathContains(parent, child string) bool {
	if parent == child {
		return true
	}
	sep := string(filepath.Separator)
	if !strings.HasSuffix(parent, sep) {
		parent += sep
	}
	return strings.HasPrefix(child, parent)
}

// SettingsJSON 生成传给 claude --settings 的内容：一个 PreToolUse 钩子，
// 每次工具调用前调用 `<ccrPath> __chain_guard`。
//
// matcher 用 catch-all（空串）而非工具名白名单：按名字匹配会漏掉没列进去的 shell
// 工具——Windows 上 claude 用 `PowerShell`（不是 `Bash`），将来可能还有 `cmd` 或别的
// 名字。空 matcher 对所有工具触发，再由 guard 内部按工具种类过滤。已对 claude
// 2.1.161 校准：空 matcher 确实对工具触发。
//
// ⚠️ 集成边界：PreToolUse settings 结构、stdin 输入字段名、用退出码 2 阻止——
// 按当前 Claude Code 钩子协议编写。实现/联调时对照 `claude` 文档核对字段，不符则改本函数与 hookInput。
func SettingsJSON(ccrPath string) string {
	return fmt.Sprintf(`{
  "hooks": {
    "PreToolUse": [
      { "matcher": "", "hooks": [ { "type": "command", "command": %q } ] }
    ]
  }
}`, ccrPath+" __chain_guard")
}

// hookInput 是 PreToolUse 钩子从 stdin 收到的（字段名按 Claude Code 协议，联调时核对）。
// 列出 chain 围栏关心的所有路径类字段：file_path（Read/Write/Edit）、path（Glob/Grep）、
// notebook_path（NotebookRead/NotebookEdit）、pattern（Glob/Grep 的模式可能含绝对路径）。
type hookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command      string `json:"command"`
		FilePath     string `json:"file_path"`
		Path         string `json:"path"`
		NotebookPath string `json:"notebook_path"`
		Pattern      string `json:"pattern"`
	} `json:"tool_input"`
}

// HookInfo 是钩子用得着的解析结果——命令 + 全部路径类字段汇总。
type HookInfo struct {
	ToolName string
	Command  string
	Paths    []string
}

// ParseHookInput 解析 PreToolUse 钩子 stdin JSON。坏 JSON 返回零值。
func ParseHookInput(raw []byte) HookInfo {
	var in hookInput
	if json.Unmarshal(raw, &in) != nil {
		return HookInfo{}
	}
	var paths []string
	for _, p := range []string{in.ToolInput.FilePath, in.ToolInput.Path, in.ToolInput.NotebookPath, in.ToolInput.Pattern} {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return HookInfo{
		ToolName: in.ToolName,
		Command:  in.ToolInput.Command,
		Paths:    paths,
	}
}

// CommandFromHookInput 从钩子 stdin JSON 里取出 Bash 命令；解析失败返回空串。
// 兼容旧调用点。
func CommandFromHookInput(raw []byte) string {
	return ParseHookInput(raw).Command
}
