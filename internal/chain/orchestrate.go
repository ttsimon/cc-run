package chain

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ttsimon/cc-run/internal/registry"
	"github.com/ttsimon/cc-run/internal/ui"
)

// Orchestrator 顺序执行一条链的各段。
type Orchestrator struct {
	reg    *registry.Registry
	Auto   bool      // true=不停顿一条道跑到黑；false=每段后在放行点征询用户
	Input  string    // 整链级需求；prompt 里 {{input}} 替换为它
	Pauser Pauser    // 放行点交互实现；默认 TermPauser
	Out    io.Writer // 隔离结果/进度输出；nil 默认 os.Stdout
	Level  Level     // 渲染详细度；默认 LevelNormal

	// runSegment 可注入以便测试；默认走真实 Runner。
	runSegment func(spec runSpec, seg Segment) (string, int, error)
}

// NewOrchestrator 返回默认编排器（真实跑 claude）。
func NewOrchestrator(reg *registry.Registry) *Orchestrator {
	o := &Orchestrator{reg: reg}
	runner := NewRunner()
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		out := o.Out
		if out == nil {
			out = os.Stdout
		}
		rnd := &Renderer{Level: o.Level, TTY: ui.WriterIsTTY(out), Out: out}
		return runner.RunSegment(spec, rnd)
	}
	o.Pauser = NewTermPauser()
	o.Out = os.Stdout
	o.Level = LevelNormal // 零值是 LevelQuiet，须显式设默认
	return o
}

// Run 顺序跑完整条链；非 Auto 时每段后在放行点征询用户。
func (o *Orchestrator) Run(c Chain) error {
	out := o.Out
	if out == nil {
		out = os.Stdout
	}
	workdir := c.Workdir
	if workdir == "" {
		workdir = "."
	}
	var iso Isolator
	if c.Isolate {
		var err error
		iso, err = newIsolator(workdir, sanitize(c.Name))
		if err != nil {
			return fmt.Errorf("建隔离区失败: %w", err)
		}
		wd, err := iso.Setup()
		if err != nil {
			return fmt.Errorf("隔离区 Setup 失败: %w", err)
		}
		workdir = wd
	}
	ccrPath := "ccr"
	if exe, err := os.Executable(); err == nil {
		ccrPath = exe
	}

	// guard 的 settings（含 PreToolUse 钩子配置）写到工作目录**之外**的临时处：
	// agent 的写操作圈在 workdir 内，够不着这里，便不能看见/篡改自己的红线钩子。
	// MkdirTemp 失败则退化为无 settings（无钩子，与原 best-effort 行为一致）。
	settingsRoot, mkErr := os.MkdirTemp("", "ccr-chain-cfg-")
	if mkErr == nil {
		defer func() { _ = os.RemoveAll(settingsRoot) }()
	} else {
		settingsRoot = ""
	}

	// 干净屋：给每段一个空的 CLAUDE_CONFIG_DIR，让无头 claude 不继承用户全局插件/技能/
	// SessionStart 钩子（superpowers 等）——它们会注入大段前言、诱发段去调技能/起子代理，
	// 行为不确定且费 token。注意 hooks 引擎本身没关，故下面 --settings 的 guard 钩子仍生效
	// （已实测：CLAUDE_CONFIG_DIR 隔离下 plugin list 为空，但 echo 仍被黑名单拦）。
	// 对比 `--bare`：那个会连我们自己的 guard 钩子一起 skip，废掉第 3 层防线，不能用。
	// MkdirTemp 失败则退化为不注入（段继承全局配置，与旧 best-effort 行为一致）。
	cleanHome, homeErr := os.MkdirTemp("", "ccr-chain-home-")
	if homeErr == nil {
		defer func() { _ = os.RemoveAll(cleanHome) }()
	} else {
		cleanHome = ""
	}

	// 任务边界（第 3 层）：追踪本链改动文件集，注入后续段 prompt 作软提示。
	// best-effort——失败则后续不注入，绝不打断链（与 segmentDiffStat 同语义）。
	tracker := newChangeTracker(workdir)
	_ = tracker.Baseline()
	relevant := map[string]bool{} // 链级只增总集

	// 据「最后一次审查」的判定决定收尾，不是「任一段曾 needs-work」——否则早段
	// needs-work + 修复段 + 末段 pass 的链会被早段粘死、误走 Abandon 丢掉成果。
	// fail-closed：只有最后一次审查明确 pass 才自动合入；needs-work 与"没产出判定"
	// （漏写/拼错 verdict）都按未通过处理——质量闸在不确定时不能静默放行。
	reviewRan := false
	lastVerdict := VerdictUnknown
	var prev string
	for i := 0; i < len(c.Segments); i++ {
		seg := c.Segments[i]

		p, err := o.reg.Resolve(seg.Profile)
		if err != nil {
			abandon(out, iso)
			return fmt.Errorf("段 #%d(%q) 的 profile 解析失败: %w", i, seg.Name, err)
		}
		// 累加上一段（已 SealSegment）的改动到链级总集，并集防"建了又删"反复。
		if files, err := tracker.ChangedFiles(); err == nil {
			for _, f := range files {
				relevant[f] = true
			}
		}
		renderedPrompt := Render(seg.Prompt, prev, o.Input)
		if seg.Review {
			renderedPrompt += ReviewInstruction()
		}
		if note := RelevantFilesNote(sortedKeys(relevant)); note != "" {
			renderedPrompt += note
		}
		// 安全：合并黑名单经 env 传给 guard；每段生成一个带 PreToolUse 钩子的 settings。
		env := map[string]string{}
		for k, v := range p.Env {
			env[k] = v
		}
		env["CCR_CHAIN_DENY"] = strings.Join(MergeDenylist(DefaultDenylist(), seg.DenyCommands), "\n")
		// 路径围栏：把 workdir 与白名单经 env 传给 guard，guard 拦绝对路径越界 / cd 上跳。
		// GIT_CEILING_DIRECTORIES 同时限 agent 自己跑的 git——非 git 目录不会被父仓库
		// 牵连（agent 在 temp/ 跑 git status 不会爬到父项目）。ceiling 是 workdir 的
		// **父目录**——git 文档语义是"不能进入这些目录"，要拦上爬就把父级列为禁区。
		// canonPath 解析软链：macOS 的 /var/folders 实为 /private/var/folders，
		// git 内部 realpath 后跟字面路径比对，不解析则 ceiling 与 PathEscapes 都失效。
		if canon := canonPath(workdir); canon != "" {
			env["CCR_CHAIN_WORKDIR"] = canon
			parent := filepath.Dir(canon)
			if parent != canon {
				env["GIT_CEILING_DIRECTORIES"] = parent
			}
		}
		if len(seg.AllowPaths) > 0 {
			env["CCR_CHAIN_ALLOW_PATHS"] = strings.Join(seg.AllowPaths, "\n")
		}
		if cleanHome != "" {
			env["CLAUDE_CONFIG_DIR"] = cleanHome
		}

		settingsPath := ""
		if settingsRoot != "" {
			settingsPath = filepath.Join(settingsRoot, "settings-"+sanitize(seg.Name)+".json")
			_ = os.WriteFile(settingsPath, []byte(SettingsJSON(ccrPath)), 0o644)
		}

		fmt.Fprintf(out, "%s 段 %d/%d %s [%s]\n",
			ui.Apply(ui.WriterIsTTY(out), ui.StyleSegment, ui.IconRun),
			i+1, len(c.Segments), seg.Name, seg.Profile)

		start := time.Now()
		spec := runSpec{
			Prompt:       renderedPrompt,
			AllowTools:   seg.AllowTools,
			Workdir:      workdir,
			SettingsPath: settingsPath,
			Env:          env,
		}
		segOut, code, err := o.runSegment(spec, seg)
		if err != nil {
			abandon(out, iso)
			return fmt.Errorf("段 #%d(%q) 启动失败: %w", i, seg.Name, err)
		}
		if code != 0 {
			abandon(out, iso)
			return fmt.Errorf("段 #%d(%q) 非 0 退出（%d），中止", i, seg.Name, code)
		}
		prev = segOut
		elapsed := time.Since(start).Round(time.Second)
		fmt.Fprintf(out, "%s 段 %d/%d 完成 (%s)\n",
			ui.Apply(ui.WriterIsTTY(out), ui.StyleOK, ui.IconOK),
			i+1, len(c.Segments), elapsed)
		if o.Auto && strings.TrimSpace(prev) != "" {
			fmt.Fprintln(out, prev) // auto 无放行点，结果在此回显
		}

		if iso != nil {
			if err := iso.SealSegment(seg.Name); err != nil {
				abandon(out, iso)
				return fmt.Errorf("段 #%d(%q) 成果固化失败: %w", i, seg.Name, err)
			}
		}
		if seg.Review {
			reviewRan = true
			lastVerdict = ReadVerdict(workdir)
		}

		// 放行点（非 Auto，且后面还有段）
		if !o.Auto && i+1 < len(c.Segments) {
			info := prev
			if seg.Review {
				switch ReadVerdict(workdir) {
				case VerdictPass:
					info += "\n[判定] pass ✓"
				case VerdictNeedsWork:
					info += "\n[判定] needs-work —— 下一段建议放行修复"
				default:
					info += "\n[判定] 未产出判定 —— 默认不自动合入，建议放行修复或检查审查段"
				}
			}
			// 只有 worktree 模式才有"上一段提交"可 diff；copydir/无隔离调了无意义，
			// 且非 git 目录调 git 会被 GIT_CEILING_DIRECTORIES 拦下，纯属噪音。
			if _, isWorktree := iso.(*worktreeIsolator); isWorktree {
				if ds := segmentDiffStat(workdir); ds != "" {
					info += "\n本段改动:\n" + ds
				}
			}
			info += fmt.Sprintf("\n耗时 %s", elapsed)
			next := c.Segments[i+1]
			d, edited, perr := o.Pauser.Pause(next, info)
			if perr != nil {
				abandon(out, iso)
				return perr
			}
			switch d {
			case DecisionQuit:
				if iso != nil {
					loc, _ := iso.Abandon()
					fmt.Fprintf(out, "已退出，成果保留在 %s\n", loc)
				}
				return nil
			case DecisionSkip:
				i++ // 跳过下一段
			case DecisionEdit:
				if edited != "" {
					c.Segments[i+1].Prompt = edited
				}
			case DecisionProceed:
			}
		}
	}

	if iso != nil {
		if reviewRan && lastVerdict != VerdictPass {
			loc, _ := iso.Abandon()
			if lastVerdict == VerdictNeedsWork {
				fmt.Fprintf(out, "审查判定 needs-work，成果未自动合入，保留在 %s\n", loc)
			} else {
				fmt.Fprintf(out, "审查未产出明确判定（pass），成果未自动合入，保留在 %s\n", loc)
			}
		} else {
			summary, err := iso.Integrate()
			if err != nil {
				loc, _ := iso.Abandon()
				fmt.Fprintf(out, "合并失败，成果保留在 %s（%v）\n", loc, err)
			} else {
				fmt.Fprintln(out, summary)
			}
		}
	}
	return nil
}

// segmentDiffStat 返回隔离 worktree 里本段相对上一提交的 diff --stat；
// 非 git / 无上一提交 / 出错时返回 ""（best-effort，不影响主流程）。
func segmentDiffStat(workdir string) string {
	out, err := gitIn(workdir, "diff", "--stat", "HEAD~1", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// abandon 在异常路径保留成果并打印取回位置（iso 为 nil 时无操作）。
func abandon(out io.Writer, iso Isolator) {
	if iso == nil {
		return
	}
	loc, _ := iso.Abandon()
	fmt.Fprintf(out, "成果保留在 %s（未合入）\n", loc)
}

// sortedKeys 返回 map 的键，已排序——给 RelevantFilesNote 稳定输出。
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
