package launcher

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/ttsimon/cc-run/internal/profile"
)

func TestComposeEnv_profile覆盖且去重(t *testing.T) {
	base := []string{"PATH=/bin", "ANTHROPIC_BASE_URL=https://old"}
	env := map[string]string{"ANTHROPIC_BASE_URL": "https://new", "FOO": "bar"}
	got := ComposeEnv(base, env)

	found := map[string]string{}
	for _, kv := range got {
		i := strings.IndexByte(kv, '=')
		found[kv[:i]] = kv[i+1:]
	}
	if found["ANTHROPIC_BASE_URL"] != "https://new" {
		t.Errorf("profile 应覆盖 base: %q", found["ANTHROPIC_BASE_URL"])
	}
	if found["PATH"] != "/bin" || found["FOO"] != "bar" {
		t.Errorf("base/新增项丢失: %+v", found)
	}
	// 去重：BASE_URL 只应出现一次
	n := 0
	for _, kv := range got {
		if strings.HasPrefix(kv, "ANTHROPIC_BASE_URL=") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("ANTHROPIC_BASE_URL 出现 %d 次，应为 1", n)
	}
}

func TestClaudeArgs(t *testing.T) {
	// 有顶层 model 且 env 未指定模型 → 追加 --model
	got := ClaudeArgs("sonnet", nil, nil, []string{"-p", "hi"})
	if strings.Join(got, " ") != "--model sonnet -p hi" {
		t.Errorf("got %v", got)
	}
	// env 已含 ANTHROPIC_MODEL → 不追加
	got = ClaudeArgs("sonnet", map[string]string{"ANTHROPIC_MODEL": "x"}, nil, []string{"-p"})
	if strings.Join(got, " ") != "-p" {
		t.Errorf("env 指定模型时不应追加 --model, got %v", got)
	}
	// 无顶层 model → 不追加
	got = ClaudeArgs("", nil, nil, []string{"chat"})
	if strings.Join(got, " ") != "chat" {
		t.Errorf("got %v", got)
	}
}

func TestClaudeArgs_接入profile默认参数(t *testing.T) {
	got := ClaudeArgs("sonnet", nil, []string{"--verbose"}, []string{"-p", "hi"})
	if strings.Join(got, " ") != "--model sonnet --verbose -p hi" {
		t.Errorf("默认参数应在 model 之后、extra 之前: %v", got)
	}
}

// TestHelperProcess 充当假的 claude：打印某环境变量并以特定码退出。
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Stdout.WriteString("BASE=" + os.Getenv("ANTHROPIC_BASE_URL"))
	os.Exit(7)
}

func TestRun_注入env并透传退出码(t *testing.T) {
	l := New()
	l.ClaudePath = os.Args[0] // 用测试二进制自身当作 claude
	l.Environ = func() []string { return append(os.Environ(), "GO_WANT_HELPER_PROCESS=1") }
	var out bytes.Buffer
	l.Stdout = &out

	p := profile.Profile{Env: map[string]string{"ANTHROPIC_BASE_URL": "https://injected"}}
	code, err := l.Run(p, []string{"-test.run=TestHelperProcess"})
	if err != nil {
		t.Fatal(err)
	}
	if code != 7 {
		t.Errorf("退出码 = %d, want 7", code)
	}
	if !strings.Contains(out.String(), "BASE=https://injected") {
		t.Errorf("env 未注入子进程, 输出: %q", out.String())
	}
}
