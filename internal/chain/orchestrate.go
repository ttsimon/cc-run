package chain

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	// 据「最后一次审查」的判定决定收尾，不是「任一段曾 needs-work」——否则早段
	// needs-work + 修复段 + 末段 pass 的链会被早段粘死、误走 Abandon 丢掉成果。
	lastReviewNeedsWork := false
	var prev string
	for i := 0; i < len(c.Segments); i++ {
		seg := c.Segments[i]

		p, err := o.reg.Resolve(seg.Profile)
		if err != nil {
			abandon(out, iso)
			return fmt.Errorf("段 #%d(%q) 的 profile 解析失败: %w", i, seg.Name, err)
		}
		renderedPrompt := Render(seg.Prompt, prev, o.Input)
		if seg.Review {
			renderedPrompt += ReviewInstruction()
		}
		// 安全：合并黑名单经 env 传给 guard；每段生成一个带 PreToolUse 钩子的 settings。
		env := map[string]string{}
		for k, v := range p.Env {
			env[k] = v
		}
		env["CCR_CHAIN_DENY"] = strings.Join(MergeDenylist(DefaultDenylist(), seg.DenyCommands), "\n")

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
			lastReviewNeedsWork = ReadVerdict(workdir) == VerdictNeedsWork
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
				}
			}
			if ds := segmentDiffStat(workdir); ds != "" {
				info += "\n本段改动:\n" + ds
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
		if lastReviewNeedsWork {
			loc, _ := iso.Abandon()
			fmt.Fprintf(out, "审查判定 needs-work，成果未自动合入，保留在 %s\n", loc)
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
