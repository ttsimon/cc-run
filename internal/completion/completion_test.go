package completion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScript_各shell含关键标记(t *testing.T) {
	cases := map[string]string{
		"bash":       "complete -F _ccr ccr",
		"zsh":        "#compdef ccr",
		"powershell": "Register-ArgumentCompleter",
	}
	for shell, marker := range cases {
		s, err := Script(shell)
		if err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		if !strings.Contains(s, marker) {
			t.Errorf("%s 脚本缺标记 %q", shell, marker)
		}
		if !strings.Contains(s, "__complete_names") {
			t.Errorf("%s 脚本应调用 __complete_names 取动态名字", shell)
		}
	}
}

func TestScript_未知shell报错(t *testing.T) {
	if _, err := Script("fish"); err == nil {
		t.Fatal("未支持的 shell 应报错")
	}
}

func TestLoadLine_带存在性兜底(t *testing.T) {
	// 引导行必须先确认 ccr 在 PATH 上才执行；否则 ccr 卸载/换版本/不在 PATH 时，
	// 开 shell 会报错弄脏启动（用户实测踩到过）。
	if got := loadLine("bash"); !strings.Contains(got, "command -v ccr") {
		t.Errorf("bash loadLine 应有存在性兜底: %q", got)
	}
	if got := loadLine("zsh"); !strings.Contains(got, "command -v ccr") {
		t.Errorf("zsh loadLine 应有存在性兜底: %q", got)
	}
	if got := loadLine("powershell"); !strings.Contains(got, "Get-Command ccr") {
		t.Errorf("powershell loadLine 应有存在性兜底: %q", got)
	}
}

func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = os.UserHomeDir })
	return home
}

func TestInstall_写入并幂等(t *testing.T) {
	home := withHome(t)
	changed, path, err := Install("bash")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("首次安装应有改动")
	}
	if filepath.Dir(path) != home {
		t.Errorf("bash rc 应在 home 下: %q", path)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), markerStart) || !strings.Contains(string(raw), "ccr completion bash") {
		t.Errorf("rc 应含引导块: %s", raw)
	}
	changed2, _, err := Install("bash")
	if err != nil {
		t.Fatal(err)
	}
	if changed2 {
		t.Error("重复安装应幂等无改动")
	}
	raw2, _ := os.ReadFile(path)
	if strings.Count(string(raw2), markerStart) != 1 {
		t.Errorf("引导块不应重复: %s", raw2)
	}
}

func TestUninstall_删干净(t *testing.T) {
	withHome(t)
	if _, _, err := Install("zsh"); err != nil {
		t.Fatal(err)
	}
	changed, path, err := Uninstall("zsh")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("卸载应有改动")
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), markerStart) {
		t.Errorf("卸载后不应残留标记: %s", raw)
	}
}
