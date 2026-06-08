// Package launcher 用某个 Profile 拉起 claude 子进程。
package launcher

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"ccr/internal/profile"
)

// ComposeEnv 把 profileEnv 叠加到 base 上（profile 覆盖同名键），去重后返回。
func ComposeEnv(base []string, profileEnv map[string]string) []string {
	merged := map[string]string{}
	for _, kv := range base {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			merged[kv[:i]] = kv[i+1:]
		}
	}
	for k, v := range profileEnv {
		merged[k] = v
	}
	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	sort.Strings(out) // 稳定输出，便于测试与调试
	return out
}

// ClaudeArgs 计算传给 claude 的参数。
// 仅当存在顶层 model 且 env 未通过 ANTHROPIC_MODEL 指定模型时，追加 --model。
func ClaudeArgs(model string, profileEnv map[string]string, extra []string) []string {
	var args []string
	_, envHasModel := profileEnv["ANTHROPIC_MODEL"]
	if model != "" && !envHasModel {
		args = append(args, "--model", model)
	}
	return append(args, extra...)
}

// Launcher 拉起 claude；各字段可注入以便测试。
type Launcher struct {
	ClaudePath string                       // 为空则在 PATH 中查找 "claude"
	LookPath   func(string) (string, error) // 默认 exec.LookPath
	Environ    func() []string              // 默认 os.Environ
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
}

// New 返回带默认值的 Launcher。
func New() *Launcher {
	return &Launcher{
		LookPath: exec.LookPath,
		Environ:  os.Environ,
		Stdin:    os.Stdin,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
	}
}

// Run 用 p 的 env 拉起 claude，透传 stdio 与退出码。
// 返回子进程退出码；仅当无法启动时返回非 nil error。
func (l *Launcher) Run(p profile.Profile, extra []string) (int, error) {
	path := l.ClaudePath
	if path == "" {
		found, err := l.LookPath("claude")
		if err != nil {
			return -1, fmt.Errorf("找不到 claude 可执行文件，请确认已安装且在 PATH 中：%w", err)
		}
		path = found
	}

	cmd := exec.Command(path, ClaudeArgs(p.Model, p.Env, extra)...)
	cmd.Env = ComposeEnv(l.Environ(), p.Env)
	cmd.Stdin = l.Stdin
	cmd.Stdout = l.Stdout
	cmd.Stderr = l.Stderr

	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil // 子进程正常退出但非 0：透传，不算 ccr 错误
	}
	return -1, err
}
