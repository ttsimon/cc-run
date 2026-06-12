package chain

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ttsimon/cc-run/internal/registry"
)

// Orchestrator 顺序执行一条链的各段。
type Orchestrator struct {
	reg    *registry.Registry
	Auto   bool      // true=不停顿一条道跑到黑；false=每段后在放行点征询用户
	Input  string    // 整链级需求；prompt 里 {{input}} 替换为它
	Pauser Pauser    // 放行点交互实现；默认 TermPauser
	Out    io.Writer // 隔离结果/进度输出；nil 默认 os.Stdout

	// runSegment 可注入以便测试；默认走真实 Runner。
	runSegment func(spec runSpec, seg Segment) (string, int, error)
}

// NewOrchestrator 返回默认编排器（真实跑 claude）。
func NewOrchestrator(reg *registry.Registry) *Orchestrator {
	o := &Orchestrator{reg: reg}
	runner := NewRunner()
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		return runner.RunSegment(spec)
	}
	o.Pauser = NewTermPauser()
	o.Out = os.Stdout
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

	needsWork := false
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
		settingsDir := filepath.Join(workdir, ".ccr-chain")
		if err := os.MkdirAll(settingsDir, 0o755); err == nil {
			settingsPath = filepath.Join(settingsDir, "settings-"+sanitize(seg.Name)+".json")
			_ = os.WriteFile(settingsPath, []byte(SettingsJSON(ccrPath)), 0o644)
		}

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

		if iso != nil {
			if err := iso.SealSegment(seg.Name); err != nil {
				abandon(out, iso)
				return fmt.Errorf("段 #%d(%q) 成果固化失败: %w", i, seg.Name, err)
			}
		}
		if seg.Review && ReadVerdict(workdir) == VerdictNeedsWork {
			needsWork = true
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
		if needsWork {
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

// abandon 在异常路径保留成果并打印取回位置（iso 为 nil 时无操作）。
func abandon(out io.Writer, iso Isolator) {
	if iso == nil {
		return
	}
	loc, _ := iso.Abandon()
	fmt.Fprintf(out, "成果保留在 %s（未合入）\n", loc)
}
