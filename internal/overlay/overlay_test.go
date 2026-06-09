package overlay

import (
	"os"
	"path/filepath"
	"testing"
)

func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = os.UserHomeDir })
	return home
}

func TestOverlay_往返(t *testing.T) {
	withHome(t)
	in := Overlay{Aliases: map[string]string{"prod": "cc-switch:DeepSeek"}, Default: "my-local"}
	if err := SaveOverlay(in); err != nil {
		t.Fatal(err)
	}
	out := LoadOverlay()
	if out.Default != "my-local" || out.Aliases["prod"] != "cc-switch:DeepSeek" {
		t.Errorf("往返不一致: %+v", out)
	}
}

func TestOverlay_缺文件回退空且别名可写(t *testing.T) {
	withHome(t)
	out := LoadOverlay()
	if out.Default != "" {
		t.Errorf("缺文件应空默认")
	}
	out.Aliases["x"] = "y" // 不应 panic（map 已初始化）
}

func TestOverlay_坏文件回退空(t *testing.T) {
	home := withHome(t)
	if err := os.MkdirAll(filepath.Join(home, ".ccr"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ccr", "overlay.json"), []byte("{bad json"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := LoadOverlay()
	if out.Default != "" || len(out.Aliases) != 0 {
		t.Errorf("坏文件应回退空")
	}
}

func TestState_往返(t *testing.T) {
	withHome(t)
	if err := SaveState(State{Last: "cc-switch:火山"}); err != nil {
		t.Fatal(err)
	}
	if LoadState().Last != "cc-switch:火山" {
		t.Errorf("state 往返失败")
	}
}
