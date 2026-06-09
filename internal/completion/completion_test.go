package completion

import (
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
