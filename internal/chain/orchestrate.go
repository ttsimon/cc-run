package chain

import (
	"fmt"

	"github.com/ttsimon/cc-run/internal/registry"
)

// Orchestrator 顺序执行一条链的各段。
type Orchestrator struct {
	reg  *registry.Registry
	Auto bool // true=不停顿一条道跑到黑（Phase C 才有停顿）

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
	return o
}

// Run 顺序跑完整条链。上段 stdout 注入下段 prompt 的 {{prev.output}}。
func (o *Orchestrator) Run(c Chain) error {
	workdir := c.Workdir
	if workdir == "" {
		workdir = "."
	}
	var prev string
	for i, seg := range c.Segments {
		p, err := o.reg.Resolve(seg.Profile)
		if err != nil {
			return fmt.Errorf("段 #%d(%q) 的 profile 解析失败: %w", i, seg.Name, err)
		}
		spec := runSpec{
			Prompt:     Render(seg.Prompt, prev),
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
	}
	return nil
}
