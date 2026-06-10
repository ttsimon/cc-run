package chain

import (
	"fmt"

	"github.com/ttsimon/cc-run/internal/registry"
)

// Orchestrator 顺序执行一条链的各段。
type Orchestrator struct {
	reg    *registry.Registry
	Auto   bool   // true=不停顿一条道跑到黑；false=每段后在放行点征询用户
	Pauser Pauser // 放行点交互实现；默认 TermPauser

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
	return o
}

// Run 顺序跑完整条链；非 Auto 时每段后在放行点征询用户。
func (o *Orchestrator) Run(c Chain) error {
	workdir := c.Workdir
	if workdir == "" {
		workdir = "."
	}
	if c.Isolate {
		wt, cleanup, err := CreateWorktree(workdir, sanitize(c.Name))
		if err != nil {
			return fmt.Errorf("隔离 worktree 失败（当前目录需为 git 仓库）: %w", err)
		}
		defer cleanup()
		workdir = wt
	}
	var prev string
	for i := 0; i < len(c.Segments); i++ {
		seg := c.Segments[i]

		p, err := o.reg.Resolve(seg.Profile)
		if err != nil {
			return fmt.Errorf("段 #%d(%q) 的 profile 解析失败: %w", i, seg.Name, err)
		}
		renderedPrompt := Render(seg.Prompt, prev)
		if seg.Review {
			renderedPrompt += ReviewInstruction()
		}
		spec := runSpec{
			Prompt:     renderedPrompt,
			AllowTools: seg.AllowTools,
			Workdir:    workdir,
			Env:        p.Env,
		}
		out, code, err := o.runSegment(spec, seg)
		if err != nil {
			return fmt.Errorf("段 #%d(%q) 启动失败: %w", i, seg.Name, err)
		}
		if code != 0 {
			return fmt.Errorf("段 #%d(%q) 非 0 退出（%d），中止", i, seg.Name, code)
		}
		prev = out

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
				return perr
			}
			switch d {
			case DecisionQuit:
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
	return nil
}
