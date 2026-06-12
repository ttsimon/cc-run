package chain

import (
	"encoding/json"
	"fmt"
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

// SettingsJSON 生成传给 claude --settings 的内容：一个 PreToolUse 钩子，
// 每次工具调用前调用 `<ccrPath> __chain_guard`。
//
// matcher 用 catch-all（空串）而非工具名白名单：按名字匹配会漏掉没列进去的 shell
// 工具——Windows 上 claude 用 `PowerShell`（不是 `Bash`），将来可能还有 `cmd` 或别的
// 名字。空 matcher 对所有工具触发，再由 guard 内部按「是否有 command」过滤：非 shell
// 工具以空 command 进来，Denied("") 恒 false，安全放行。已对 claude 2.1.161 校准：
// 空 matcher 确实对工具触发。
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
type hookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// CommandFromHookInput 从钩子 stdin JSON 里取出 Bash 命令；解析失败返回空串。
func CommandFromHookInput(raw []byte) string {
	var in hookInput
	if json.Unmarshal(raw, &in) != nil {
		return ""
	}
	return in.ToolInput.Command
}
