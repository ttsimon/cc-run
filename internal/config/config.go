// Package config 解析 ccr 的路径配置：env > ~/.ccr/config.json > 默认。
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// userHomeDir 便于测试替换。
var userHomeDir = os.UserHomeDir

// Config 是解析后的运行时路径。
type Config struct {
	DB          string // cc-switch 库路径
	ProfilesDir string // 自定义 profiles 目录
}

// fileConfig 对应 ~/.ccr/config.json。
type fileConfig struct {
	DB          string `json:"db"`
	ProfilesDir string `json:"profilesDir"`
}

// Load 按优先级解析配置。任何缺失项回退到下一级。
func Load() Config {
	home, err := userHomeDir()
	if err != nil {
		home = "."
	}
	defDB := filepath.Join(home, ".cc-switch", "cc-switch.db")
	defProfiles := filepath.Join(home, ".ccr", "profiles")

	var fc fileConfig
	if raw, err := os.ReadFile(filepath.Join(home, ".ccr", "config.json")); err == nil {
		_ = json.Unmarshal(raw, &fc) // 坏文件忽略，回退默认
	}

	pick := func(envKey, fileVal, def string) string {
		if v := os.Getenv(envKey); v != "" {
			return v
		}
		if fileVal != "" {
			return fileVal
		}
		return def
	}

	return Config{
		DB:          pick("CCR_DB", fc.DB, defDB),
		ProfilesDir: pick("CCR_PROFILES_DIR", fc.ProfilesDir, defProfiles),
	}
}
