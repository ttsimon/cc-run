package chain

import (
	"bytes"
	"os"
	"path/filepath"
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
	wd, _ := os.Getwd()
	os.Stdout.WriteString("PROMPT=" + prompt + "\nBASE=" + os.Getenv("ANTHROPIC_BASE_URL") + "\nCWD=" + wd)
	os.Exit(0)
}

func TestRunSegment_在Workdir里运行(t *testing.T) {
	// isolate 隔离的关键：子进程 cwd 必须是 spec.Workdir（worktree），
	// 否则相对路径文件会落到调用者真实仓库里。
	dir := t.TempDir()
	want, err := filepath.EvalSymlinks(dir) // macOS 下 TempDir 可能含符号链接
	if err != nil {
		want = dir
	}

	r := NewRunner()
	r.ClaudePath = os.Args[0]
	r.Environ = func() []string { return append(os.Environ(), "GO_WANT_HELPER_PROCESS=1") }
	var errOut bytes.Buffer
	r.Stderr = &errOut

	out, code, err := r.RunSegment(runSpec{
		Prompt:    "x",
		Workdir:   dir,
		ExtraArgs: []string{"-test.run=TestHelperProcess", "--"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("退出码 %d", code)
	}
	if !strings.Contains(out, "CWD="+want) {
		t.Errorf("子进程应在 Workdir(%q) 运行，实际输出: %q", want, out)
	}
}

func TestRunSegment_捕获stdout并注入env(t *testing.T) {
	r := NewRunner()
	r.ClaudePath = os.Args[0]
	r.Environ = func() []string { return append(os.Environ(), "GO_WANT_HELPER_PROCESS=1") }
	var errOut bytes.Buffer
	r.Stderr = &errOut

	out, code, err := r.RunSegment(runSpec{
		Prompt: "你好",
		Env:    map[string]string{"ANTHROPIC_BASE_URL": "https://injected"},
		// "-test.run" 指定 helper；"--" 让 flag 包停止解析，
		// 使 SegmentArgs 产生的 -p / --output-format 等旗标落入 os.Args 非旗标区。
		ExtraArgs: []string{"-test.run=TestHelperProcess", "--"},
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
