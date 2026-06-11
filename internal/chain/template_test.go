package chain

import "testing"

func TestRender_替换prev输出(t *testing.T) {
	got := Render("读 {{prev.output}} 照做", "计划在 docs/plans/x.md", "")
	if got != "读 计划在 docs/plans/x.md 照做" {
		t.Errorf("got %q", got)
	}
}

func TestRender_容忍空格变体(t *testing.T) {
	got := Render("X {{ prev.output }} Y", "Z", "")
	if got != "X Z Y" {
		t.Errorf("应容忍花括号内空格: %q", got)
	}
}

func TestRender_无占位原样返回(t *testing.T) {
	if Render("没有占位", "忽略", "忽略") != "没有占位" {
		t.Error("无占位应原样")
	}
}

func TestRender_替换input(t *testing.T) {
	got := Render("需求是 {{input}} 去做", "", "给登录页加记住我")
	if got != "需求是 给登录页加记住我 去做" {
		t.Errorf("应替换 {{input}}: %q", got)
	}
}

func TestRender_input容忍空格变体(t *testing.T) {
	got := Render("X {{ input }} Y", "", "需求")
	if got != "X 需求 Y" {
		t.Errorf("应容忍 {{ input }} 内空格: %q", got)
	}
}

func TestRender_input与prev同段各替各的(t *testing.T) {
	got := Render("需求 {{input}}；上段 {{prev.output}}", "PREV", "REQ")
	if got != "需求 REQ；上段 PREV" {
		t.Errorf("input 与 prev 应各替各的: %q", got)
	}
}
