package chain

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSegmentArgs_基本(t *testing.T) {
	got := SegmentArgs("照做", []string{"Read", "Write"}, "/tmp/wd", "")
	j := strings.Join(got, " ")
	for _, want := range []string{"-p", "照做", "--allowedTools", "Read,Write", "--add-dir", "/tmp/wd", "--output-format", "text"} {
		if !strings.Contains(j, want) {
			t.Errorf("缺 %q: %v", want, got)
		}
	}
}

func TestSegmentArgs_带settings(t *testing.T) {
	got := SegmentArgs("x", nil, "/wd", "/tmp/settings.json")
	if !strings.Contains(strings.Join(got, " "), "--settings /tmp/settings.json") {
		t.Errorf("应含 --settings: %v", got)
	}
}

func TestSegmentArgs_无allowtools不加旗标(t *testing.T) {
	got := SegmentArgs("x", nil, "/wd", "")
	if strings.Contains(strings.Join(got, " "), "--allowedTools") {
		t.Errorf("无 allow_tools 不应加 --allowedTools: %v", got)
	}
}

// TestHelperProcess 充当假 claude：把 -p 后的 prompt 原样打印，并打印一个 env 值。
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	var prompt string
	for i, a := range args {
		if a == "-p" && i+1 < len(args) {
			prompt = args[i+1]
		}
	}
	os.Stdout.WriteString("PROMPT=" + prompt + "\nBASE=" + os.Getenv("ANTHROPIC_BASE_URL"))
	os.Exit(0)
}

func TestRunSegment_捕获stdout并注入env(t *testing.T) {
	r := NewRunner()
	r.ClaudePath = os.Args[0]
	r.Environ = func() []string { return append(os.Environ(), "GO_WANT_HELPER_PROCESS=1") }
	var errOut bytes.Buffer
	r.Stderr = &errOut

	out, code, err := r.RunSegment(runSpec{
		Prompt:    "你好",
		Env:       map[string]string{"ANTHROPIC_BASE_URL": "https://injected"},
		ExtraArgs: []string{"-test.run=TestHelperProcess"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("退出码 %d", code)
	}
	if !strings.Contains(out, "PROMPT=你好") || !strings.Contains(out, "BASE=https://injected") {
		t.Errorf("应捕获到 stdout 并注入 env: %q", out)
	}
}
