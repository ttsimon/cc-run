package chain

import (
	"strings"
	"testing"
)

func renderAll(level Level, evs ...Event) string {
	var b strings.Builder
	r := &Renderer{Level: level, TTY: false, Out: &b}
	for _, e := range evs {
		r.Render(e)
	}
	return b.String()
}

func TestRenderer_quiet不打工具行(t *testing.T) {
	out := renderAll(LevelQuiet,
		Event{Kind: EventToolUse, Tool: "Write", Target: "a.html"},
		Event{Kind: EventAssistantText, Text: "思考"},
	)
	if strings.Contains(out, "Write") || strings.Contains(out, "思考") {
		t.Errorf("quiet 不应打工具/思考行: %q", out)
	}
}

func TestRenderer_normal打工具行不打思考(t *testing.T) {
	out := renderAll(LevelNormal,
		Event{Kind: EventToolUse, Tool: "Write", Target: "a.html"},
		Event{Kind: EventAssistantText, Text: "思考内容"},
	)
	if !strings.Contains(out, "Write") || !strings.Contains(out, "a.html") {
		t.Errorf("normal 应打工具行: %q", out)
	}
	if strings.Contains(out, "思考内容") {
		t.Errorf("normal 不应打思考行: %q", out)
	}
}

func TestRenderer_verbose打思考与token(t *testing.T) {
	out := renderAll(LevelVerbose,
		Event{Kind: EventAssistantText, Text: "思考内容"},
		Event{Kind: EventResult, Text: "答案", Usage: "in 10 / out 5"},
	)
	if !strings.Contains(out, "思考内容") {
		t.Errorf("verbose 应打思考行: %q", out)
	}
	if !strings.Contains(out, "in 10 / out 5") {
		t.Errorf("verbose 应打 token: %q", out)
	}
}

func TestRenderer_非TTY无ANSI(t *testing.T) {
	out := renderAll(LevelNormal, Event{Kind: EventToolUse, Tool: "Bash", Target: "ls"})
	if strings.Contains(out, "\x1b[") {
		t.Errorf("非 TTY 不应含 ANSI: %q", out)
	}
}
