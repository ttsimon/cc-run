package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ttsimon/cc-run/internal/profile"
	"github.com/ttsimon/cc-run/internal/registry"
)

func reg() *registry.Registry {
	return registry.New([]profile.Profile{
		{Name: "DeepSeek", Source: profile.SourceCustom, Model: "sonnet",
			Env:     map[string]string{"ANTHROPIC_AUTH_TOKEN": "sk-FAKEcli1234567890", "ANTHROPIC_BASE_URL": "https://api.deepseek.com/anthropic"},
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
	if strings.Contains(s, "sk-FAKEcli1234567890") {
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
	if strings.Contains(s, "sk-FAKEcli1234567890") {
		t.Errorf("默认 show 不应出现完整 token: %q", s)
	}
	if !strings.Contains(s, "sk-FAKE…") {
		t.Errorf("show 应出现打码 token: %q", s)
	}
}

func TestCmdShow_reveal显示完整(t *testing.T) {
	var out bytes.Buffer
	cmdShow(reg(), "DeepSeek", true, &out)
	if !strings.Contains(out.String(), "sk-FAKEcli1234567890") {
		t.Errorf("--reveal 应显示完整 token")
	}
}

func TestCmdShow_未知名字非零退出(t *testing.T) {
	var out bytes.Buffer
	if code := cmdShow(reg(), "nope", false, &out); code == 0 {
		t.Error("未知名字应非零退出")
	}
}

func TestRunAlias_设置并落盘(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	profDir := filepath.Join(home, ".ccr", "profiles")
	if err := os.MkdirAll(profDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profDir, "my-local.json"),
		[]byte(`{"env":{"ANTHROPIC_BASE_URL":"http://x"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CCR_DB", filepath.Join(home, "none.db"))
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
	if code := Execute([]string{"alias", "x", "does-not-exist"}); code == 0 {
		t.Error("坏目标应非 0 退出")
	}
}

func TestRunChain_缺文件参数报错(t *testing.T) {
	if code := Execute([]string{"chain"}); code == 0 {
		t.Error("chain 缺文件应非 0")
	}
}

func TestRunChain_文件不存在报错(t *testing.T) {
	if code := Execute([]string{"chain", "no-such-file.yaml"}); code == 0 {
		t.Error("文件不存在应非 0")
	}
}

func TestRunChainGuard_命中退2(t *testing.T) {
	t.Setenv("CCR_CHAIN_DENY", "rm -rf\ngit push")
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`)
	var errBuf bytes.Buffer
	if code := runChainGuard(in, &errBuf); code != 2 {
		t.Errorf("命中应退 2, got %d", code)
	}
}

func TestRunChainGuard_未命中退0(t *testing.T) {
	t.Setenv("CCR_CHAIN_DENY", "rm -rf")
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls -la"}}`)
	var errBuf bytes.Buffer
	if code := runChainGuard(in, &errBuf); code != 0 {
		t.Errorf("未命中应退 0, got %d", code)
	}
}

func TestRunChain_init生成模板(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir) // 切到临时目录，避免污染仓库
	if code := Execute([]string{"chain", "init"}); code != 0 {
		t.Fatalf("chain init 应成功, code=%d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "plan-impl-review.chain.yaml")); err != nil {
		t.Errorf("应生成模板文件: %v", err)
	}
}

func TestRunChain_init未知模板报错(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if code := Execute([]string{"chain", "init", "nope"}); code == 0 {
		t.Error("未知模板应非 0")
	}
}
