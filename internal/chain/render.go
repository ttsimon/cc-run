package chain

import (
	"fmt"
	"io"
	"strings"

	"github.com/ttsimon/cc-run/internal/ui"
)

// Level 是运行时详细度。
type Level int

const (
	LevelQuiet   Level = iota // -q：仅段框 + 最终结果
	LevelNormal               // 默认：+ 逐工具调用
	LevelVerbose              // -v：+ 思考文本 + token
)

// Renderer 按级别与 TTY 把事件渲染到 Out。result 文本不在此打印——
// 由 orchestrator 在段尾决定如何展示（交棒/段框）。
type Renderer struct {
	Level Level
	TTY   bool
	Out   io.Writer
}

func (r *Renderer) Render(e Event) {
	if r == nil || r.Out == nil {
		return
	}
	switch e.Kind {
	case EventToolUse:
		if r.Level >= LevelNormal {
			line := fmt.Sprintf("  %s %s %s", ui.IconTool, e.Tool, e.Target)
			fmt.Fprintln(r.Out, ui.Apply(r.TTY, ui.StyleDim, strings.TrimRight(line, " ")))
		}
	case EventAssistantText:
		if r.Level >= LevelVerbose {
			fmt.Fprintln(r.Out, ui.Apply(r.TTY, ui.StyleDim, "  "+strings.TrimSpace(e.Text)))
		}
	case EventResult:
		if r.Level >= LevelVerbose && e.Usage != "" {
			fmt.Fprintln(r.Out, ui.Apply(r.TTY, ui.StyleDim, "  [tokens] "+e.Usage))
		}
	}
}
