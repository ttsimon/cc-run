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
	"strings"

	"github.com/ttsimon/cc-run/internal/chain"
	"github.com/ttsimon/cc-run/internal/completion"
	"github.com/ttsimon/cc-run/internal/config"
	"github.com/ttsimon/cc-run/internal/doctor"
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
		case "doctor":
			return runDoctor(cfg, args[1:], os.Stdout)
		case "__chain_guard":
			return runChainGuard(os.Stdin, os.Stderr)
		case "chain":
			return runChain(cfg, args[1:], os.Stdout)
		case "completion":
			return runCompletion(args[1:], os.Stdout)
		case "alias":
			return runAlias(cfg, args[1:], os.Stdout)
		case "unalias":
			return runUnalias(args[1:], os.Stdout)
		case "default":
			return runDefault(cfg, args[1:], os.Stdout)
		case "__complete_names":
			return cmdCompleteNames(cfg, os.Stdout)
		}
	}

	// 其余：ccr <name> [claude 参数...]、ccr -（上次）、ccr .（默认）、或 ccr（交互）。
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
		ov := overlay.LoadOverlay()
		query := args[0]
		extra = args[1:]

		// 特殊记号翻译。
		switch query {
		case "-":
			last := overlay.LoadState().Last
			if last == "" {
				fmt.Fprintln(os.Stderr, "还没有「上次」记录；先用 `ccr <名字>` 跑一次。")
				return 1
			}
			query = last
		case ".":
			if ov.Default == "" {
				fmt.Fprintln(os.Stderr, "还没设默认；用 `ccr default <名字>` 设置。")
				return 1
			}
			query = ov.Default
		}

		res, err := r.Lookup(query, ov.Aliases)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if len(res.Candidates) > 0 {
			p, err := tui.SelectProfile(res.Candidates)
			if err != nil {
				fmt.Fprintln(os.Stderr, "已取消")
				return 1
			}
			chosen = p
		} else {
			chosen = res.Profile
		}
	}

	// 记录「上次」（限定名，便于 `ccr -` 重放）。失败不致命。
	_ = overlay.SaveState(overlay.State{Last: fmt.Sprintf("%s:%s", chosen.Source, chosen.Name)})

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

// runAlias: 无参列出；`<别名> <目标>` 设置（校验目标可解析）。
func runAlias(cfg config.Config, args []string, out io.Writer) int {
	ov := overlay.LoadOverlay()
	if len(args) == 0 {
		keys := make([]string, 0, len(ov.Aliases))
		for k := range ov.Aliases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(out, "%-16s -> %s\n", k, ov.Aliases[k])
		}
		return 0
	}
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: ccr alias <别名> <目标profile>")
		return 1
	}
	alias, target := args[0], args[1]
	r, code := buildRegistry(cfg)
	if code != 0 {
		return code
	}
	if _, err := r.Resolve(target); err != nil {
		fmt.Fprintln(os.Stderr, "别名目标无法解析:", err)
		return 1
	}
	ov.Aliases[alias] = target
	if err := overlay.SaveOverlay(ov); err != nil {
		fmt.Fprintln(os.Stderr, "保存失败:", err)
		return 1
	}
	fmt.Fprintf(out, "已设别名: %s -> %s\n", alias, target)
	return 0
}

// runUnalias: 删一个别名。
func runUnalias(args []string, out io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "用法: ccr unalias <别名>")
		return 1
	}
	ov := overlay.LoadOverlay()
	if _, ok := ov.Aliases[args[0]]; !ok {
		fmt.Fprintf(os.Stderr, "没有别名 %q\n", args[0])
		return 1
	}
	delete(ov.Aliases, args[0])
	if err := overlay.SaveOverlay(ov); err != nil {
		fmt.Fprintln(os.Stderr, "保存失败:", err)
		return 1
	}
	fmt.Fprintf(out, "已删别名: %s\n", args[0])
	return 0
}

// runDefault: 无参打印当前默认；`<目标>` 设置（校验可解析）。
func runDefault(cfg config.Config, args []string, out io.Writer) int {
	ov := overlay.LoadOverlay()
	if len(args) == 0 {
		if ov.Default == "" {
			fmt.Fprintln(out, "（未设默认）用 `ccr default <名字>` 设置，之后 `ccr .` 直启。")
		} else {
			fmt.Fprintln(out, ov.Default)
		}
		return 0
	}
	target := args[0]
	r, code := buildRegistry(cfg)
	if code != 0 {
		return code
	}
	if _, err := r.Resolve(target); err != nil {
		fmt.Fprintln(os.Stderr, "默认目标无法解析:", err)
		return 1
	}
	ov.Default = target
	if err := overlay.SaveOverlay(ov); err != nil {
		fmt.Fprintln(os.Stderr, "保存失败:", err)
		return 1
	}
	fmt.Fprintf(out, "已设默认: %s（`ccr .` 直启）\n", target)
	return 0
}

// runDoctor: 无参体检全部 profile；带名只检一个。
func runDoctor(cfg config.Config, args []string, out io.Writer) int {
	r, code := buildRegistry(cfg)
	if code != 0 {
		return code
	}
	var targets []profile.Profile
	if len(args) > 0 {
		p, err := r.Resolve(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		targets = []profile.Profile{p}
	} else {
		targets = r.List()
	}
	allOK := true
	for _, p := range targets {
		res := doctor.Check(p)
		mark := "✓"
		if !res.OK {
			mark = "✗"
			allOK = false
		}
		fmt.Fprintf(out, "%s %-20s %s\n", mark, res.Name, res.Detail)
	}
	if !allOK {
		return 1
	}
	return 0
}

// runChain: ccr chain <file> [--auto]
func runChain(cfg config.Config, args []string, out io.Writer) int {
	auto := false
	var file string
	for _, a := range args {
		switch a {
		case "--auto":
			auto = true
		default:
			file = a
		}
	}
	if file == "" {
		fmt.Fprintln(os.Stderr, "用法: ccr chain <chain.yaml> [--auto]")
		return 1
	}
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "读不到 chain 文件:", err)
		return 1
	}
	c, err := chain.Parse(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	r, code := buildRegistry(cfg)
	if code != 0 {
		return code
	}
	o := chain.NewOrchestrator(r)
	o.Auto = auto
	if err := o.Run(c); err != nil {
		fmt.Fprintln(os.Stderr, "chain 失败:", err)
		return 1
	}
	fmt.Fprintln(out, "chain 完成:", c.Name)
	return 0
}

// runChainGuard: PreToolUse 钩子调用；命中黑名单（CCR_CHAIN_DENY 换行分隔）则退 2 阻止。
func runChainGuard(in io.Reader, errOut io.Writer) int {
	raw, _ := io.ReadAll(in)
	cmd := chain.CommandFromHookInput(raw)
	denylist := strings.Split(os.Getenv("CCR_CHAIN_DENY"), "\n")
	if chain.Denied(cmd, denylist) {
		fmt.Fprintf(errOut, "ccr: 命令命中红线黑名单，已拦截：%s\n", cmd)
		return 2
	}
	return 0
}

// printUsage 打印帮助。
func printUsage(out io.Writer) {
	fmt.Fprint(out, `ccr — 用选定 provider 的环境变量启动 claude

用法:
  ccr                          交互式选择一个配置并启动
  ccr <名字|别名|前缀> [claude参数]  按名/别名/模糊命中启动，多余参数透传给 claude
  ccr -                        重跑上次用的配置
  ccr .                        跑默认配置（先 ccr default 设过）
  ccr ls                       列出所有配置（两来源）
  ccr show <名字> [--reveal]    查看某配置（默认 token 打码）
  ccr edit <名字>              用 $EDITOR 编辑/新建自定义配置
  ccr alias [<别名> <目标>]     列出 / 设置别名
  ccr unalias <别名>           删除别名
  ccr default [<名字>]          查看 / 设置默认配置
  ccr completion <shell>       打印补全脚本（bash/zsh/powershell）
  ccr completion install [shell] [--uninstall]
                               一键装/卸补全到当前 shell 配置

配置来源: cc-switch 库 + 自定义目录（~/.ccr/profiles/*.json）
元数据:   别名/默认存 ~/.ccr/overlay.json，上次用的存 ~/.ccr/state.json
`)
}
