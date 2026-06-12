package chain

import (
	"bufio"
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
	args := []string{"-p", prompt, "--output-format", "stream-json", "--verbose", "--add-dir", workdir}
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

// RunSegment 跑一段：流式解析 claude 的 stream-json，逐事件喂给 rnd 渲染，
// 返回 (最终 result 文本, 退出码, error)。rnd 为 nil 时不渲染、仅抽取结果。
// 无 result 事件时回退为整段 stdout（防脆）。
func (r *Runner) RunSegment(spec runSpec, rnd *Renderer) (string, int, error) {
	path := r.ClaudePath
	if path == "" {
		found, err := r.LookPath("claude")
		if err != nil {
			return "", -1, fmt.Errorf("找不到 claude 可执行文件：%w", err)
		}
		path = found
	}

	args := append(spec.ExtraArgs, SegmentArgs(spec.Prompt, spec.AllowTools, spec.Workdir, spec.SettingsPath)...)

	cmd := exec.Command(path, args...)
	cmd.Dir = spec.Workdir
	cmd.Env = launcher.ComposeEnv(r.Environ(), spec.Env)
	cmd.Stderr = r.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", -1, fmt.Errorf("接管 stdout 失败: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", -1, fmt.Errorf("启动 claude 失败: %w", err)
	}

	var resultText string
	var rawAll strings.Builder
	reader := bufio.NewReader(stdout)
	for {
		line, rerr := reader.ReadString('\n')
		if len(line) > 0 {
			rawAll.WriteString(line)
			for _, e := range ParseEventLine([]byte(line)) {
				rnd.Render(e) // Renderer.Render 对 nil 接收者安全
				if e.Kind == EventResult {
					resultText = e.Text
				}
			}
		}
		if rerr != nil {
			break
		}
	}

	out := resultText
	if out == "" {
		out = strings.TrimSpace(rawAll.String()) // 防脆回退：无 result 事件
	}

	werr := cmd.Wait()
	if werr == nil {
		return out, 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(werr, &exitErr) {
		return out, exitErr.ExitCode(), nil
	}
	return out, -1, werr
}
