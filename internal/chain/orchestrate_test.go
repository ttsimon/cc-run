package chain

import (
	"strings"
	"testing"

	"github.com/ttsimon/cc-run/internal/profile"
	"github.com/ttsimon/cc-run/internal/registry"
)

func testReg() *registry.Registry {
	return registry.New([]profile.Profile{
		{Name: "strong", Source: profile.SourceCCSwitch, Env: map[string]string{"ANTHROPIC_BASE_URL": "http://a"}},
		{Name: "cheap", Source: profile.SourceCustom, Env: map[string]string{"ANTHROPIC_BASE_URL": "http://b"}},
	})
}

func TestOrchestrate_顺序跑并交棒(t *testing.T) {
	c := Chain{
		Name: "t",
		Segments: []Segment{
			{Name: "a", Profile: "strong", Prompt: "first"},
			{Name: "b", Profile: "cheap", Prompt: "用了上段: {{prev.output}}"},
		},
	}
	var seenPrompts []string
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		seenPrompts = append(seenPrompts, spec.Prompt)
		return seg.Name + "-out", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if len(seenPrompts) != 2 {
		t.Fatalf("应跑 2 段, got %d", len(seenPrompts))
	}
	if seenPrompts[0] != "first" {
		t.Errorf("段0 prompt 错: %q", seenPrompts[0])
	}
	if !strings.Contains(seenPrompts[1], "a-out") {
		t.Errorf("段1 应注入上段输出: %q", seenPrompts[1])
	}
}

func TestOrchestrate_未知profile报错(t *testing.T) {
	c := Chain{Segments: []Segment{{Name: "a", Profile: "nope", Prompt: "x"}}}
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { return "", 0, nil }
	if err := o.Run(c); err == nil {
		t.Error("未知 profile 应报错")
	}
}

func TestOrchestrate_段非零退出即停(t *testing.T) {
	c := Chain{Segments: []Segment{
		{Name: "a", Profile: "strong", Prompt: "x"},
		{Name: "b", Profile: "cheap", Prompt: "y"},
	}}
	calls := 0
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		calls++
		return "", 3, nil
	}
	if err := o.Run(c); err == nil {
		t.Error("段非 0 退出应中止并报错")
	}
	if calls != 1 {
		t.Errorf("应在第一段失败后停, calls=%d", calls)
	}
}

// fakePauser 按预设序列返回决策。
type fakePauser struct {
	seq  []Decision
	edit string
	i    int
}

func (f *fakePauser) Pause(_ Segment, _ string) (Decision, string, error) {
	d := DecisionProceed
	if f.i < len(f.seq) {
		d = f.seq[f.i]
	}
	f.i++
	return d, f.edit, nil
}

func TestOrchestrate_退出在放行点中止(t *testing.T) {
	c := Chain{Segments: []Segment{
		{Name: "a", Profile: "strong", Prompt: "x"},
		{Name: "b", Profile: "cheap", Prompt: "y"},
	}}
	calls := 0
	o := NewOrchestrator(testReg())
	o.Auto = false
	o.Pauser = &fakePauser{seq: []Decision{DecisionQuit}}
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { calls++; return "out", 0, nil }
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("退出后不应跑第二段, calls=%d", calls)
	}
}

func TestOrchestrate_跳过略过下一段(t *testing.T) {
	c := Chain{Segments: []Segment{
		{Name: "a", Profile: "strong", Prompt: "x"},
		{Name: "b", Profile: "cheap", Prompt: "y"},
		{Name: "c", Profile: "strong", Prompt: "z"},
	}}
	var ran []string
	o := NewOrchestrator(testReg())
	o.Auto = false
	o.Pauser = &fakePauser{seq: []Decision{DecisionSkip, DecisionProceed}}
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { ran = append(ran, seg.Name); return "o", 0, nil }
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if strings.Join(ran, ",") != "a,c" {
		t.Errorf("应跑 a,c（跳过 b）, got %v", ran)
	}
}
