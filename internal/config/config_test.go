package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_默认值基于home(t *testing.T) {
	home := t.TempDir()
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = os.UserHomeDir })
	t.Setenv("CCR_DB", "")
	t.Setenv("CCR_PROFILES_DIR", "")

	c := Load()
	if c.DB != filepath.Join(home, ".cc-switch", "cc-switch.db") {
		t.Errorf("DB 默认值错误: %q", c.DB)
	}
	if c.ProfilesDir != filepath.Join(home, ".ccr", "profiles") {
		t.Errorf("ProfilesDir 默认值错误: %q", c.ProfilesDir)
	}
}

func TestLoad_环境变量覆盖(t *testing.T) {
	home := t.TempDir()
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = os.UserHomeDir })
	t.Setenv("CCR_DB", "/x/my.db")
	t.Setenv("CCR_PROFILES_DIR", "/x/profiles")

	c := Load()
	if c.DB != "/x/my.db" || c.ProfilesDir != "/x/profiles" {
		t.Errorf("env 未覆盖: %+v", c)
	}
}

func TestLoad_配置文件覆盖默认但低于env(t *testing.T) {
	home := t.TempDir()
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = os.UserHomeDir })
	if err := os.MkdirAll(filepath.Join(home, ".ccr"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(home, ".ccr", "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"db":"/from/file.db","profilesDir":"/from/file/profiles"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CCR_DB", "")                    // 不设 → 用文件值
	t.Setenv("CCR_PROFILES_DIR", "/env/wins") // 设了 → 覆盖文件

	c := Load()
	if c.DB != "/from/file.db" {
		t.Errorf("DB 应取文件值, got %q", c.DB)
	}
	if c.ProfilesDir != "/env/wins" {
		t.Errorf("ProfilesDir 应取 env 值, got %q", c.ProfilesDir)
	}
}
