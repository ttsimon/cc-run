package chain

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/ttsimon/cc-run/internal/launcher"
)

// SegmentArgs 构造传给 claude 的参数：headless 打印模式 + 工具白名单 + 目录范围 + 钩子设置。
// settingsPath 为空时不加 --settings；allowTools 为空时不加 --allowedTools。
//
// ⚠️ 集成边界：这些旗标名按当前 Claude Code CLI 编写。实现时跑一次
// `claude --help` 确认 -p / --allowedTools / --add-dir / --output-format / --settings 存在，
// 版本不同则只改这里的常量字符串。
func SegmentArgs(prompt string, allowTools []string, workdir, settingsPath string) []string {
	args := []string{"-p", prompt, "--output-format", "text", "--add-dir", workdir}
	if len(allowTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(allowTools, ","))
	}
	if settingsPath != "" {
		args = append(args, "--settings", settingsPath)
	}
	return args
}

// runSpec 是跑一段所需的全部输入。
type runSpec struct {
	Prompt       string
	AllowTools   []string
	Workdir      string
	SettingsPath string
	Env          map[string]string // 已解析 profile 的 env
	ExtraArgs    []string          // 仅测试注入；真实运行为空
}

// Runner 用某 profile 拉起一次无头 claude，捕获其 stdout 作为本段产出。
type Runner struct {
	ClaudePath string
	LookPath   func(string) (string, error)
	Environ    func() []string
	Stderr     io.Writer
}

// NewRunner 返回带默认值的 Runner。
func NewRunner() *Runner {
	return &Runner{
		LookPath: exec.LookPath,
		Environ:  os.Environ,
		Stderr:   os.Stderr,
	}
}

// RunSegment 跑一段，返回 (stdout 文本, 退出码, error)。
// 仅当无法启动 claude 时返回 error；子进程非 0 退出通过 code 透传。
func (r *Runner) RunSegment(spec runSpec) (string, int, error) {
	path := r.ClaudePath
	if path == "" {
		found, err := r.LookPath("claude")
		if err != nil {
			return "", -1, fmt.Errorf("找不到 claude 可执行文件：%w", err)
		}
		path = found
	}

	// ExtraArgs（仅测试注入）放在最前，确保 Go helper-process 模式中
	// -test.run 旗标先于非测试旗标被解析。真实运行 ExtraArgs 为空，无影响。
	args := append(spec.ExtraArgs, SegmentArgs(spec.Prompt, spec.AllowTools, spec.Workdir, spec.SettingsPath)...)

	cmd := exec.Command(path, args...)
	cmd.Env = launcher.ComposeEnv(r.Environ(), spec.Env)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = r.Stderr

	err := cmd.Run()
	if err == nil {
		return out.String(), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out.String(), exitErr.ExitCode(), nil
	}
	return out.String(), -1, err
}
