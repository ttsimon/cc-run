package chain

import (
	"os"
	"path/filepath"
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

func TestOrchestrate_review段追加指令(t *testing.T) {
	c := Chain{Segments: []Segment{{Name: "r", Profile: "strong", Prompt: "审查", Review: true}}}
	var seen string
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { seen = spec.Prompt; return "o", 0, nil }
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen, ".ccr-chain/verdict") {
		t.Errorf("review 段 prompt 应被追加判定指令: %q", seen)
	}
}

func TestOrchestrate_可选段auto下照跑(t *testing.T) {
	c := Chain{Segments: []Segment{
		{Name: "impl", Profile: "cheap", Prompt: "x"},
		{Name: "fix", Profile: "cheap", Prompt: "y", Optional: true},
	}}
	var ran []string
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { ran = append(ran, seg.Name); return "o", 0, nil }
	o.Run(c)
	if strings.Join(ran, ",") != "impl,fix" {
		t.Errorf("auto 下可选段应照跑: %v", ran)
	}
}

func TestOrchestrate_可选段非auto默认跳过(t *testing.T) {
	c := Chain{Segments: []Segment{
		{Name: "impl", Profile: "cheap", Prompt: "x"},
		{Name: "fix", Profile: "cheap", Prompt: "y", Optional: true},
	}}
	var ran []string
	o := NewOrchestrator(testReg())
	o.Auto = false
	o.Pauser = &fakePauser{seq: []Decision{DecisionSkip}}
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { ran = append(ran, seg.Name); return "o", 0, nil }
	o.Run(c)
	if strings.Join(ran, ",") != "impl" {
		t.Errorf("非 auto 跳过可选段后只应跑 impl: %v", ran)
	}
}

// capturePauser 记录它收到的展示文本。
type capturePauser struct {
	got string
	d   Decision
}

func (c *capturePauser) Pause(_ Segment, prevOutput string) (Decision, string, error) {
	c.got = prevOutput
	return c.d, "", nil
}

func TestOrchestrate_isolate跑在worktree(t *testing.T) {
	repo := initRepo(t)
	var seenWorkdir string
	c := Chain{
		Name:     "t",
		Isolate:  true,
		Workdir:  repo,
		Segments: []Segment{{Name: "a", Profile: "strong", Prompt: "x"}},
	}
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		seenWorkdir = spec.Workdir
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if seenWorkdir == repo || seenWorkdir == "." {
		t.Errorf("isolate 时段应跑在 worktree 而非原目录: %q", seenWorkdir)
	}
	// worktree 在 Run 结束时已被 cleanup 移除
	if _, err := os.Stat(seenWorkdir); !os.IsNotExist(err) {
		t.Errorf("Run 结束后 worktree 应已清理: %q", seenWorkdir)
	}
}

func TestOrchestrate_放行点展示判定(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".ccr-chain"), 0o755)
	os.WriteFile(filepath.Join(dir, ".ccr-chain", "verdict"), []byte("needs-work"), 0o644)
	c := Chain{
		Workdir: dir,
		Segments: []Segment{
			{Name: "review", Profile: "strong", Prompt: "审", Review: true},
			{Name: "fix", Profile: "cheap", Prompt: "改", Optional: true},
		},
	}
	cp := &capturePauser{d: DecisionProceed}
	o := NewOrchestrator(testReg())
	o.Auto = false
	o.Pauser = cp
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { return "o", 0, nil }
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cp.got, "needs-work") {
		t.Errorf("放行点应展示判定: %q", cp.got)
	}
}
