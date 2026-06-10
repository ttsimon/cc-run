package chain

import "testing"

func TestRender_替换prev输出(t *testing.T) {
	got := Render("读 {{prev.output}} 照做", "计划在 docs/plans/x.md")
	if got != "读 计划在 docs/plans/x.md 照做" {
		t.Errorf("got %q", got)
	}
}

func TestRender_容忍空格变体(t *testing.T) {
	got := Render("X {{ prev.output }} Y", "Z")
	if got != "X Z Y" {
		t.Errorf("应容忍花括号内空格: %q", got)
	}
}

func TestRender_无占位原样返回(t *testing.T) {
	if Render("没有占位", "忽略") != "没有占位" {
		t.Error("无占位应原样")
	}
}
