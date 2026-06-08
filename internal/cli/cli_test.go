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
			Env:     map[string]string{"ANTHROPIC_AUTH_TOKEN": "sk-1234567890", "ANTHROPIC_BASE_URL": "https://api.deepseek.com/anthropic"},
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
