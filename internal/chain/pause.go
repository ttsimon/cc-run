package chain

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Decision 是放行点用户的选择。
type Decision int

const (
	DecisionProceed Decision = iota // 放行下一段
	DecisionSkip                    // 跳过下一段
	DecisionQuit                    // 退出整条链
	DecisionEdit                    // 改下一段的指令后再放行
)

// parseDecision 把用户输入映射为决策；空/y=放行。
func parseDecision(s string) Decision {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "s", "skip":
		return DecisionSkip
	case "q", "quit":
		return DecisionQuit
	case "e", "edit":
		return DecisionEdit
	default:
		return DecisionProceed
	}
}

// Pauser 在段间征询用户决策；prevOutput 是刚跑完那段的产出（供展示）。
// 返回决策；DecisionEdit 时第二个返回值是用户改写的新指令。
type Pauser interface {
	Pause(nextSeg Segment, prevOutput string) (Decision, string, error)
}

// TermPauser 在终端打印决策提示并读一行。
type TermPauser struct {
	In  io.Reader
	Out io.Writer
}

// NewTermPauser 默认接 os.Stdin/os.Stdout。
func NewTermPauser() *TermPauser { return &TermPauser{In: os.Stdin, Out: os.Stdout} }

func (t *TermPauser) Pause(nextSeg Segment, prevOutput string) (Decision, string, error) {
	fmt.Fprintf(t.Out, "\n⏸  上段产出：\n%s\n", prevOutput)
	fmt.Fprintf(t.Out, "下一段：%s（profile=%s）\n", nextSeg.Name, nextSeg.Profile)
	fmt.Fprint(t.Out, "[回车=放行 / s=跳过 / e=改指令 / q=退出] > ")
	line, _ := bufio.NewReader(t.In).ReadString('\n')
	d := parseDecision(line)
	if d == DecisionEdit {
		fmt.Fprint(t.Out, "输入新指令（单行）> ")
		edited, _ := bufio.NewReader(t.In).ReadString('\n')
		return d, edited, nil
	}
	return d, "", nil
}
