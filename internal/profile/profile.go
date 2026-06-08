// Package profile 定义可启动的 Claude 配置及其脱敏辅助。
package profile

import "strings"

// Source 标识一个 Profile 的来源。
type Source string

const (
	SourceCCSwitch Source = "cc-switch"
	SourceCustom   Source = "custom"
)

// Profile 是一份可启动的 Claude 配置。
type Profile struct {
	Name      string            // provider name 或自定义文件名(去扩展名)
	Source    Source            // 来源
	Model     string            // settings_config 顶层 "model"，可空
	Env       map[string]string // 注入 claude 的环境变量
	BaseURL   string            // Env["ANTHROPIC_BASE_URL"]，仅展示用
	IsCurrent bool              // 仅 cc-switch：是否为当前激活 provider
}

// secretKeySubstrings 命中其一即视为敏感值，需打码。
var secretKeySubstrings = []string{"TOKEN", "KEY", "SECRET", "PASSWORD"}

// RedactToken 保留前 7 个字符，其余用省略号代替。
func RedactToken(s string) string {
	if s == "" {
		return ""
	}
	const keep = 7
	r := []rune(s)
	if len(r) <= keep {
		return "…"
	}
	return string(r[:keep]) + "…"
}

// isSecretKey 判断某环境变量名是否为敏感项。
func isSecretKey(k string) bool {
	up := strings.ToUpper(k)
	for _, s := range secretKeySubstrings {
		if strings.Contains(up, s) {
			return true
		}
	}
	return false
}

// RedactEnv 返回一份副本，其中敏感项的值被打码。
func RedactEnv(env map[string]string) map[string]string {
	out := make(map[string]string, len(env))
	for k, v := range env {
		if isSecretKey(k) {
			out[k] = RedactToken(v)
		} else {
			out[k] = v
		}
	}
	return out
}
