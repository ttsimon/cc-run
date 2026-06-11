package chain

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseDecision(t *testing.T) {
	cases := map[string]Decision{
		"":  DecisionProceed,
		"y": DecisionProceed,
		"s": DecisionSkip,
		"q": DecisionQuit,
		"e": DecisionEdit,
	}
	for in, want := range cases {
		if got := parseDecision(in); got != want {
			t.Errorf("parseDecision(%q)=%v want %v", in, got, want)
		}
	}
}

func TestTermPauser_edit去除换行(t *testing.T) {
	in := strings.NewReader("e\n  新指令内容  \n")
	var out bytes.Buffer
	p := &TermPauser{In: in, Out: &out}
	d, edited, err := p.Pause(Segment{Name: "x", Profile: "y"}, "上段输出")
	if err != nil {
		t.Fatal(err)
	}
	if d != DecisionEdit {
		t.Fatalf("应为 Edit, got %v", d)
	}
	if strings.ContainsAny(edited, "\r\n") {
		t.Errorf("edited 不应含换行: %q", edited)
	}
	if strings.TrimSpace(edited) != "新指令内容" {
		t.Errorf("edited 内容错: %q", edited)
	}
}

func TestTermPauser_可选段标注(t *testing.T) {
	in := strings.NewReader("\n") // 回车放行
	var out bytes.Buffer
	p := &TermPauser{In: in, Out: &out}
	if _, _, err := p.Pause(Segment{Name: "fix", Profile: "y", Optional: true}, "x"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "可选") {
		t.Errorf("可选段应在提示里标注「可选」: %q", out.String())
	}
}
