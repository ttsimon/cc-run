// Package cli 解析参数并把各层装配起来。
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/ttsimon/cc-run/internal/completion"
	"github.com/ttsimon/cc-run/internal/config"
	"github.com/ttsimon/cc-run/internal/launcher"
	"github.com/ttsimon/cc-run/internal/overlay"
	"github.com/ttsimon/cc-run/internal/profile"
	"github.com/ttsimon/cc-run/internal/registry"
	"github.com/ttsimon/cc-run/internal/source"
	"github.com/ttsimon/cc-run/internal/tui"
)

// 版本信息，由 main 注入（GoReleaser 通过 ldflags 设置）。
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Execute 是 CLI 入口，返回进程退出码。
func Execute(args []string) int {
	cfg := config.Load()

	// 子命令分发。
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help":
			printUsage(os.Stdout)
			return 0
		case "-v", "--version", "version":
			fmt.Printf("ccr %s (commit %s, built %s)\n", Version, Commit, Date)
			return 0
		case "ls":
			r, code := buildRegistry(cfg)
			if code != 0 {
				return code
			}
			return cmdLs(r, os.Stdout)
		case "show":
			return runShow(cfg, args[1:])
		case "edit":
			return runEdit(cfg, args[1:])
		case "completion":
			return runCompletion(args[1:], os.Stdout)
		case "__complete_names":
			return cmdCompleteNames(cfg, os.Stdout)
		}
	}

	// 其余：ccr <name> [claude 参数...] 或 ccr（交互）。
	r, code := buildRegistry(cfg)
	if code != 0 {
		return code
	}

	var chosen profile.Profile
	var extra []string
	if len(args) == 0 {
		p, err := tui.SelectProfile(r.List())
		if err != nil {
			fmt.Fprintln(os.Stderr, "已取消")
			return 1
		}
		chosen = p
	} else {
		p, err := r.Resolve(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		chosen = p
		extra = args[1:]
	}

	code2, err := launcher.New().Run(chosen, extra)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return code2
}

// buildRegistry 加载两来源并合并；两来源都空时给出引导。
func buildRegistry(cfg config.Config) (*registry.Registry, int) {
	profiles, errs := source.LoadAll(
		source.NewCCSwitch(cfg.DB),
		source.NewCustomDir(cfg.ProfilesDir),
	)
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, "警告:", e)
	}
	if len(profiles) == 0 {
		fmt.Fprintf(os.Stderr,
			"没有找到任何配置。\n未检测到 cc-switch，或没有自定义 profile。\n"+
				"可在 %s 下放一个 JSON，例如 deepseek.json：\n"+
				"  {\"model\":\"sonnet\",\"env\":{\"ANTHROPIC_BASE_URL\":\"...\",\"ANTHROPIC_AUTH_TOKEN\":\"...\"}}\n"+
				"或运行 `ccr edit deepseek` 直接创建。\n",
			cfg.ProfilesDir)
		return nil, 1
	}
	return registry.New(profiles), 0
}

// cmdLs 打印所有 profile（不泄露 token）。
func cmdLs(r *registry.Registry, out io.Writer) int {
	for _, p := range r.List() {
		cur := " "
		if p.IsCurrent {
			cur = "●"
		}
		fmt.Fprintf(out, "%s %-20s [%-9s] %-10s %s\n", cur, p.Name, p.Source, p.Model, p.BaseURL)
	}
	return 0
}

// runShow 解析 show 的参数后调用 cmdShow。
func runShow(cfg config.Config, args []string) int {
	reveal := false
	var name string
	for _, a := range args {
		if a == "--reveal" {
			reveal = true
		} else {
			name = a
		}
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, "用法: ccr show <名字> [--reveal]")
		return 1
	}
	r, code := buildRegistry(cfg)
	if code != 0 {
		return code
	}
	return cmdShow(r, name, reveal, os.Stdout)
}

// cmdShow 打印某 profile 的完整内容；reveal=false 时 token 打码。
func cmdShow(r *registry.Registry, name string, reveal bool, out io.Writer) int {
	p, err := r.Resolve(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	env := p.Env
	if !reveal {
		env = profile.RedactEnv(p.Env)
	}
	fmt.Fprintf(out, "名字:   %s\n来源:   %s\n模型:   %s\n", p.Name, p.Source, p.Model)
	fmt.Fprintln(out, "环境变量:")
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(out, "  %s=%s\n", k, env[k])
	}
	return 0
}

// runEdit 用 $EDITOR 打开/新建自定义 profile 的 JSON 文件。
func runEdit(cfg config.Config, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "用法: ccr edit <名字>")
		return 1
	}
	name := args[0]
	path := filepath.Join(cfg.ProfilesDir, name+".json")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(cfg.ProfilesDir, 0o700); err != nil {
			fmt.Fprintln(os.Stderr, "创建目录失败:", err)
			return 1
		}
		tmpl, _ := json.MarshalIndent(map[string]any{
			"model": "",
			"env": map[string]string{
				"ANTHROPIC_BASE_URL":   "",
				"ANTHROPIC_AUTH_TOKEN": "",
			},
		}, "", "  ")
		if err := os.WriteFile(path, tmpl, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, "写入模板失败:", err)
			return 1
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "vi"
		}
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "编辑器退出异常:", err)
		return 1
	}
	fmt.Println("已保存:", path)
	return 0
}

// cmdCompleteNames 打印补全用的名字：profile 名 + 别名键，每行一个。
// 供补全脚本调用；任何缺失都安静处理，始终退出 0。
func cmdCompleteNames(cfg config.Config, out io.Writer) int {
	profiles, _ := source.LoadAll(
		source.NewCCSwitch(cfg.DB),
		source.NewCustomDir(cfg.ProfilesDir),
	)
	for _, p := range profiles {
		fmt.Fprintln(out, p.Name)
	}
	for alias := range overlay.LoadOverlay().Aliases {
		fmt.Fprintln(out, alias)
	}
	return 0
}

// runCompletion 处理 `ccr completion ...`。
func runCompletion(args []string, out io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "用法: ccr completion <bash|zsh|powershell> | ccr completion install [shell] [--uninstall]")
		return 1
	}

	if args[0] == "install" {
		uninstall := false
		shell := ""
		for _, a := range args[1:] {
			if a == "--uninstall" {
				uninstall = true
			} else {
				shell = a
			}
		}
		if shell == "" {
			shell = completion.DetectShell()
		}
		if shell == "" {
			fmt.Fprintln(os.Stderr, "无法探测当前 shell，请显式指定：ccr completion install <bash|zsh|powershell>")
			return 1
		}
		if uninstall {
			changed, path, err := completion.Uninstall(shell)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			if changed {
				fmt.Fprintf(out, "已从 %s 移除补全。重开终端生效。\n", path)
			} else {
				fmt.Fprintf(out, "%s 中未发现补全，无需移除。\n", path)
			}
			return 0
		}
		changed, path, err := completion.Install(shell)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if changed {
			fmt.Fprintf(out, "已把补全写入 %s。重开终端或 source 它生效。\n", path)
		} else {
			fmt.Fprintf(out, "%s 已包含补全，无需重复。\n", path)
		}
		return 0
	}

	// 否则 args[0] 当作 shell，打印脚本。
	script, err := completion.Script(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprint(out, script)
	return 0
}

// printUsage 打印帮助。
func printUsage(out io.Writer) {
	fmt.Fprint(out, `ccr — 用选定 provider 的环境变量启动 claude

用法:
  ccr                      交互式选择一个配置并启动
  ccr <名字> [claude参数]   按名字直启，多余参数透传给 claude
  ccr ls                   列出所有配置（两来源）
  ccr show <名字> [--reveal] 查看某配置（默认 token 打码）
  ccr edit <名字>           用 $EDITOR 编辑/新建自定义配置

配置来源: cc-switch 库 + 自定义目录（~/.ccr/profiles/*.json）
`)
}
