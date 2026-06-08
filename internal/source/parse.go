// Package source 把各来源的原始数据解析为 profile.Profile。
package source

import (
	"encoding/json"
	"fmt"

	"github.com/ttsimon/cc-run/internal/profile"
)

// settingsConfig 对应 cc-switch settings_config 与自定义文件的 JSON 形状。
type settingsConfig struct {
	Model string            `json:"model"`
	Env   map[string]string `json:"env"`
}

// ParseSettingsConfig 将一段 settings_config JSON 解析为 Profile。
func ParseSettingsConfig(name string, src profile.Source, raw string) (profile.Profile, error) {
	var sc settingsConfig
	if err := json.Unmarshal([]byte(raw), &sc); err != nil {
		return profile.Profile{}, fmt.Errorf("解析 %q 的配置失败: %w", name, err)
	}
	if sc.Env == nil {
		sc.Env = map[string]string{}
	}
	return profile.Profile{
		Name:    name,
		Source:  src,
		Model:   sc.Model,
		Env:     sc.Env,
		BaseURL: sc.Env["ANTHROPIC_BASE_URL"],
	}, nil
}
