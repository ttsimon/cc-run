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
