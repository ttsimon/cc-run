// Package chain 把一份 yaml 描述的多后端流水线解析、校验并编排执行。
package chain

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Chain 是一条流水线。
type Chain struct {
	Name     string    `yaml:"name"`
	Isolate  bool      `yaml:"isolate"` // true 则在临时 git worktree 跑，可整体回滚
	Workdir  string    `yaml:"workdir"` // 工作目录，空=当前目录
	Segments []Segment `yaml:"segments"`
}

// Segment 是一段：用某 provider 跑一次无头 claude。
type Segment struct {
	Name         string   `yaml:"name"`
	Profile      string   `yaml:"profile"`       // ccr profile 查询名（registry 解析）
	Prompt       string   `yaml:"prompt"`        // 可含 {{prev.output}}
	AllowTools   []string `yaml:"allow_tools"`   // claude --allowedTools
	DenyCommands []string `yaml:"deny_commands"` // 追加到内置命令黑名单
	Review       bool     `yaml:"review"`        // 该段产出 findings + 判定
	Optional     bool     `yaml:"optional"`      // 仅在放行点用户选择时才跑（如修复段）
}

// Parse 解析并校验一条链。
func Parse(data []byte) (Chain, error) {
	var c Chain
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Chain{}, fmt.Errorf("解析 chain yaml 失败: %w", err)
	}
	if err := c.Validate(); err != nil {
		return Chain{}, err
	}
	return c, nil
}

// Validate 校验基本约束。
func (c Chain) Validate() error {
	if len(c.Segments) == 0 {
		return fmt.Errorf("chain 至少要有一个段（segments 为空）")
	}
	for i, s := range c.Segments {
		if s.Profile == "" {
			return fmt.Errorf("段 #%d(%q) 缺 profile", i, s.Name)
		}
		if s.Prompt == "" {
			return fmt.Errorf("段 #%d(%q) 缺 prompt", i, s.Name)
		}
	}
	return nil
}
