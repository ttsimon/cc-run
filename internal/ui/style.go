// Package ui 提供 ccr 各命令共享的终端样式：调色板、符号集与 TTY 降级助手。
package ui

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

// 符号集（段框/工具行/状态）。
const (
	IconTool  = "🔧"
	IconOK    = "✔"
	IconRun   = "▶"
	IconPause = "⏸"
)

// 调色板（仅在 TTY 时经 Apply 生效）。
var (
	StyleSegment = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	StyleOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	StyleErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	StyleDim     = lipgloss.NewStyle().Faint(true)
)

// Apply 仅在 tty 为真时给 s 套样式，否则原样返回（非 TTY/管道/CI 降级）。
func Apply(tty bool, st lipgloss.Style, s string) string {
	if !tty {
		return s
	}
	return st.Render(s)
}

// IsTTY 报告 f 是否连到终端。
func IsTTY(f *os.File) bool {
	return isatty.IsTerminal(f.Fd())
}

// WriterIsTTY 在 w 是 *os.File 且为终端时返回 true；否则 false（含 bytes.Buffer 等）。
func WriterIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && IsTTY(f)
}
