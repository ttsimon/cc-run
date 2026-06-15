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
		Name:    "t",
		Workdir: t.TempDir(),
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

func TestOrchestrate_注入input到prompt(t *testing.T) {
	c := Chain{
		Workdir:  t.TempDir(),
		Segments: []Segment{{Name: "a", Profile: "strong", Prompt: "做 {{input}} 这个需求"}},
	}
	var seen string
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.Input = "给登录页加记住我"
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { seen = spec.Prompt; return "o", 0, nil }
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if seen != "做 给登录页加记住我 这个需求" {
		t.Errorf("段 prompt 应注入 input: %q", seen)
	}
}

func TestOrchestrate_未知profile报错(t *testing.T) {
	c := Chain{Workdir: t.TempDir(), Segments: []Segment{{Name: "a", Profile: "nope", Prompt: "x"}}}
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { return "", 0, nil }
	if err := o.Run(c); err == nil {
		t.Error("未知 profile 应报错")
	}
}

func TestOrchestrate_段非零退出即停(t *testing.T) {
	c := Chain{Workdir: t.TempDir(), Segments: []Segment{
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
	c := Chain{Workdir: t.TempDir(), Segments: []Segment{
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
	c := Chain{Workdir: t.TempDir(), Segments: []Segment{
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
	c := Chain{Workdir: t.TempDir(), Segments: []Segment{{Name: "r", Profile: "strong", Prompt: "审查", Review: true}}}
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
	c := Chain{Workdir: t.TempDir(), Segments: []Segment{
		{Name: "impl", Profile: "cheap", Prompt: "x"},
		{Name: "fix", Profile: "cheap", Prompt: "y", Optional: true},
	}}
	var ran []string
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { ran = append(ran, seg.Name); return "o", 0, nil }
	_ = o.Run(c)
	if strings.Join(ran, ",") != "impl,fix" {
		t.Errorf("auto 下可选段应照跑: %v", ran)
	}
}

func TestOrchestrate_可选段非auto默认跳过(t *testing.T) {
	c := Chain{Workdir: t.TempDir(), Segments: []Segment{
		{Name: "impl", Profile: "cheap", Prompt: "x"},
		{Name: "fix", Profile: "cheap", Prompt: "y", Optional: true},
	}}
	var ran []string
	o := NewOrchestrator(testReg())
	o.Auto = false
	o.Pauser = &fakePauser{seq: []Decision{DecisionSkip}}
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { ran = append(ran, seg.Name); return "o", 0, nil }
	_ = o.Run(c)
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

// 事故回归：agent 没提交就结束，成果绝不能丢——isolate 成功应把产物合回原仓库。
func TestOrchestrate_isolate成功合回成果(t *testing.T) {
	repo := initRepo(t)
	c := Chain{
		Name:     "t",
		Isolate:  true,
		Workdir:  repo,
		Segments: []Segment{{Name: "impl", Profile: "strong", Prompt: "x"}},
	}
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.Out = &strings.Builder{}
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		_ = os.WriteFile(filepath.Join(spec.Workdir, "out.txt"), []byte("产物"), 0o644)
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(repo, "out.txt")); string(b) != "产物" {
		t.Errorf("成功应把成果合回原仓库, got %q", b)
	}
}

func TestOrchestrate_isolate_needswork不合回(t *testing.T) {
	repo := initRepo(t)
	c := Chain{
		Name:    "t",
		Isolate: true,
		Workdir: repo,
		Segments: []Segment{
			{Name: "review", Profile: "strong", Prompt: "审", Review: true},
		},
	}
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.Out = &strings.Builder{}
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		_ = os.WriteFile(filepath.Join(spec.Workdir, "out.txt"), []byte("半成品"), 0o644)
		cd := filepath.Join(spec.Workdir, ".ccr-chain")
		_ = os.MkdirAll(cd, 0o755)
		_ = os.WriteFile(filepath.Join(cd, "verdict"), []byte("needs-work"), 0o644)
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "out.txt")); !os.IsNotExist(err) {
		t.Errorf("needs-work 不应把成果合回原仓库")
	}
}

func TestOrchestrate_isolate_段报错保留成果(t *testing.T) {
	repo := initRepo(t)
	c := Chain{
		Name:     "t",
		Isolate:  true,
		Workdir:  repo,
		Segments: []Segment{{Name: "impl", Profile: "strong", Prompt: "x"}},
	}
	o := NewOrchestrator(testReg())
	o.Auto = true
	out := &strings.Builder{}
	o.Out = out
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		_ = os.WriteFile(filepath.Join(spec.Workdir, "out.txt"), []byte("产物"), 0o644)
		return "", 7, nil
	}
	if err := o.Run(c); err == nil {
		t.Fatal("段非 0 退出应报错")
	}
	if !strings.Contains(out.String(), "保留") {
		t.Errorf("报错应打印成果保留位置, got %q", out.String())
	}
}

// 多审查段：早段 needs-work、末段 pass，应据末次判定合回（不能被早段 needs-work 粘死）。
func TestOrchestrate_isolate_末次审查pass则合回(t *testing.T) {
	repo := initRepo(t)
	c := Chain{
		Name:    "t",
		Isolate: true,
		Workdir: repo,
		Segments: []Segment{
			{Name: "r1", Profile: "strong", Prompt: "审1", Review: true},
			{Name: "r2", Profile: "strong", Prompt: "审2", Review: true},
		},
	}
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.Out = &strings.Builder{}
	verdicts := []string{"needs-work", "pass"}
	n := 0
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		_ = os.WriteFile(filepath.Join(spec.Workdir, "out.txt"), []byte("成品"), 0o644)
		cd := filepath.Join(spec.Workdir, ".ccr-chain")
		_ = os.MkdirAll(cd, 0o755)
		_ = os.WriteFile(filepath.Join(cd, "verdict"), []byte(verdicts[n]), 0o644)
		n++
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(repo, "out.txt")); string(b) != "成品" {
		t.Errorf("末次审查 pass 应把成果合回原仓库, got %q", b)
	}
}

// 铁律回归：isolate 下用户在放行点退出，成果不合回但临时分支必须保留可取回。
func TestOrchestrate_isolate_退出保留分支(t *testing.T) {
	repo := initRepo(t)
	c := Chain{
		Name:    "t",
		Isolate: true,
		Workdir: repo,
		Segments: []Segment{
			{Name: "a", Profile: "strong", Prompt: "x"},
			{Name: "b", Profile: "cheap", Prompt: "y"},
		},
	}
	o := NewOrchestrator(testReg())
	o.Auto = false
	o.Pauser = &fakePauser{seq: []Decision{DecisionQuit}}
	out := &strings.Builder{}
	o.Out = out
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		_ = os.WriteFile(filepath.Join(spec.Workdir, "out.txt"), []byte("半成品"), 0o644)
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "out.txt")); !os.IsNotExist(err) {
		t.Error("quit 不应把成果合回原仓库工作树")
	}
	br, _ := gitIn(repo, "branch", "--list", "ccr-chain/*")
	if strings.TrimSpace(string(br)) == "" {
		t.Error("quit 后临时分支应保留以便取回成果（铁律）")
	}
	if !strings.Contains(out.String(), "保留") {
		t.Errorf("quit 应打印成果保留位置, got %q", out.String())
	}
}

func TestOrchestrate_段带settings与黑名单env(t *testing.T) {
	dir := t.TempDir()
	c := Chain{Workdir: dir, Segments: []Segment{
		{Name: "a", Profile: "strong", Prompt: "x", DenyCommands: []string{"custom-bad"}},
	}}
	var seen runSpec
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { seen = spec; return "o", 0, nil }
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if seen.SettingsPath == "" {
		t.Error("应生成 settings 路径")
	}
	if !strings.Contains(seen.Env["CCR_CHAIN_DENY"], "rm -rf") {
		t.Error("env 应含默认黑名单")
	}
	if !strings.Contains(seen.Env["CCR_CHAIN_DENY"], "custom-bad") {
		t.Error("env 应含段追加项")
	}
}

// 路径围栏：每段 spec.Env 必须给 guard 提供 workdir 与 GIT_CEILING_DIRECTORIES，
// 且 workdir 是绝对路径——guard 用它做 PathEscapes 基准；GIT_CEILING_DIRECTORIES
// 防 agent 跑 git 时被父仓库牵连。
func TestOrchestrate_注入workdir围栏env(t *testing.T) {
	dir := t.TempDir()
	c := Chain{Workdir: dir, Segments: []Segment{
		{Name: "a", Profile: "strong", Prompt: "x", AllowPaths: []string{"/tmp"}},
	}}
	var seen runSpec
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { seen = spec; return "o", 0, nil }
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(seen.Env["CCR_CHAIN_WORKDIR"]) {
		t.Errorf("CCR_CHAIN_WORKDIR 应为绝对路径, got %q", seen.Env["CCR_CHAIN_WORKDIR"])
	}
	// GIT_CEILING_DIRECTORIES 必须是 workdir 的父目录——git "ceiling" 语义是"不能进入"，
	// 要拦上爬就把父级列为禁区。
	if got, want := seen.Env["GIT_CEILING_DIRECTORIES"], filepath.Dir(seen.Env["CCR_CHAIN_WORKDIR"]); got != want {
		t.Errorf("GIT_CEILING_DIRECTORIES 应为 workdir 的父目录: got %q want %q", got, want)
	}
	if !strings.Contains(seen.Env["CCR_CHAIN_ALLOW_PATHS"], "/tmp") {
		t.Errorf("AllowPaths 应经 env 传给 guard, got %q", seen.Env["CCR_CHAIN_ALLOW_PATHS"])
	}
}

// 回归：非 git 目录在父级是 git 仓库时，gitIn 不应爬到父级——这正是 chain 在 temp/
// 跑出父仓库 diff 的根因。GIT_CEILING_DIRECTORIES=dir 强制锁住。
func TestGitIn_非git子目录不爬父仓库(t *testing.T) {
	repo := initRepo(t)
	sub := filepath.Join(repo, "non-git-child")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := gitIn(sub, "rev-parse", "--is-inside-work-tree")
	if err == nil && strings.TrimSpace(string(out)) == "true" {
		t.Errorf("非 git 子目录不应被父仓库牵连, got %q", out)
	}
}

func TestOrchestrate_放行点加厚含耗时(t *testing.T) {
	c := Chain{Workdir: t.TempDir(), Segments: []Segment{
		{Name: "a", Profile: "strong", Prompt: "x"},
		{Name: "b", Profile: "cheap", Prompt: "y"},
	}}
	cp := &capturePauser{d: DecisionProceed}
	o := NewOrchestrator(testReg())
	o.Auto = false
	o.Pauser = cp
	o.Out = &strings.Builder{}
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { return "o", 0, nil }
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cp.got, "耗时") {
		t.Errorf("放行点应含耗时: %q", cp.got)
	}
}

func TestOrchestrate_打印段框(t *testing.T) {
	var out strings.Builder
	c := Chain{Workdir: t.TempDir(), Segments: []Segment{{Name: "impl", Profile: "strong", Prompt: "x"}}}
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.Out = &out
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) { return "o", 0, nil }
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "impl") || !strings.Contains(s, "1/1") {
		t.Errorf("应打印段框（名+进度）: %q", s)
	}
}

func TestOrchestrate_Level默认normal(t *testing.T) {
	o := NewOrchestrator(testReg())
	if o.Level != LevelNormal {
		t.Errorf("默认级别应为 normal, got %v", o.Level)
	}
}

// guard 的 settings 含 PreToolUse 钩子配置，不该让 agent 看见/篡改：必须写在
// 工作目录之外（agent 写操作圈在 workdir 内，够不着），且 Run 结束后清理。
func TestOrchestrate_settings在工作目录外(t *testing.T) {
	dir := t.TempDir()
	c := Chain{Workdir: dir, Segments: []Segment{{Name: "a", Profile: "strong", Prompt: "x"}}}
	var seenPath string
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		seenPath = spec.SettingsPath
		if spec.SettingsPath == "" {
			t.Error("应生成 settings 路径")
		} else {
			if _, err := os.Stat(spec.SettingsPath); err != nil {
				t.Errorf("段执行时 settings 应存在: %v", err)
			}
			if strings.HasPrefix(spec.SettingsPath, dir) {
				t.Errorf("settings 不应落在工作目录内: %q (workdir=%q)", spec.SettingsPath, dir)
			}
		}
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".ccr-chain", "settings-a.json")); !os.IsNotExist(err) {
		t.Error("工作目录里不应残留 settings 文件")
	}
	if seenPath != "" {
		if _, err := os.Stat(seenPath); !os.IsNotExist(err) {
			t.Errorf("Run 后临时 settings 应已清理: %q", seenPath)
		}
	}
}

func TestOrchestrate_注入相关文件集到后续段(t *testing.T) {
	dir := t.TempDir() // 非 git → fsTracker
	c := Chain{
		Workdir: dir,
		Segments: []Segment{
			{Name: "plan", Profile: "strong", Prompt: "规划"},
			{Name: "impl", Profile: "strong", Prompt: "实现"},
			{Name: "review", Profile: "cheap", Prompt: "审查"},
		},
	}
	var seenPrompts []string
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		seenPrompts = append(seenPrompts, spec.Prompt)
		if seg.Name == "impl" {
			_ = os.WriteFile(filepath.Join(dir, "foo.go"), []byte("x"), 0o644)
			_ = os.WriteFile(filepath.Join(dir, "bar.go"), []byte("y"), 0o644)
		}
		return seg.Name + "-out", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(seenPrompts[0], "本次链已改动") {
		t.Errorf("首段不应有文件清单: %q", seenPrompts[0])
	}
	if !strings.Contains(seenPrompts[2], "foo.go") || !strings.Contains(seenPrompts[2], "bar.go") {
		t.Errorf("review 段应含相关文件清单: %q", seenPrompts[2])
	}
}

func TestOrchestrate_tracker出错不打断链(t *testing.T) {
	c := Chain{
		Workdir: t.TempDir(),
		Segments: []Segment{
			{Name: "a", Profile: "strong", Prompt: "x"},
			{Name: "b", Profile: "strong", Prompt: "y"},
		},
	}
	o := NewOrchestrator(testReg())
	o.Auto = true
	ran := 0
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		ran++
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if ran != 2 {
		t.Errorf("两段都应跑完, ran=%d", ran)
	}
}

func TestOrchestrate_放行点展示判定(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".ccr-chain"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".ccr-chain", "verdict"), []byte("needs-work"), 0o644)
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

// 干净屋：每段必须注入一个独立、空的临时 CLAUDE_CONFIG_DIR，避免无头 claude 继承
// 用户全局插件/技能/SessionStart 钩子（superpowers 等）——它们会注入大段前言、诱发
// 段去调技能/起子代理，行为不确定且费 token。hooks 引擎本身没关，故 --settings 的
// guard 钩子仍生效（已实测：echo 仍被黑名单拦）。必须落在工作目录之外。
func TestOrchestrate_注入干净CLAUDE_CONFIG_DIR(t *testing.T) {
	dir := t.TempDir()
	c := Chain{Workdir: dir, Segments: []Segment{{Name: "a", Profile: "strong", Prompt: "x"}}}
	var cfg string
	var existedDuringRun bool
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		cfg = spec.Env["CLAUDE_CONFIG_DIR"]
		// 必须在段执行时就存在（Run 结束会 defer 清理，故得在回调里查）
		if info, err := os.Stat(cfg); err == nil && info.IsDir() {
			existedDuringRun = true
		}
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(cfg) {
		t.Errorf("应注入绝对路径的 CLAUDE_CONFIG_DIR, got %q", cfg)
	}
	if !existedDuringRun {
		t.Errorf("CLAUDE_CONFIG_DIR 段执行时应是已存在的目录: %q", cfg)
	}
	if strings.HasPrefix(cfg, dir) {
		t.Errorf("CLAUDE_CONFIG_DIR 不应落在工作目录内: %q (workdir=%q)", cfg, dir)
	}
}

// fail-closed：审查段没产出可识别判定（漏写/拼错 verdict，或被带跑没写）时，绝不能
// 默认合入——质量闸在"不确定"时必须按未通过处理，而非静默放行。
func TestOrchestrate_isolate_审查无判定不合回(t *testing.T) {
	repo := initRepo(t)
	c := Chain{
		Name:    "t",
		Isolate: true,
		Workdir: repo,
		Segments: []Segment{
			{Name: "review", Profile: "strong", Prompt: "审", Review: true},
		},
	}
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.Out = &strings.Builder{}
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		_ = os.WriteFile(filepath.Join(spec.Workdir, "out.txt"), []byte("半成品"), 0o644)
		// 故意不写 .ccr-chain/verdict —— 模拟 agent 漏写
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "out.txt")); !os.IsNotExist(err) {
		t.Errorf("审查段未产出判定时不应把成果合回原仓库（fail-closed）")
	}
}

// 放行点：审查段没产出判定时要明示（默认不合入），不能让用户以为"没消息=通过"。
func TestOrchestrate_放行点提示无判定(t *testing.T) {
	dir := t.TempDir() // 无 verdict 文件
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
	if !strings.Contains(cp.got, "未产出判定") {
		t.Errorf("放行点应提示审查未产出判定: %q", cp.got)
	}
}

func TestOrchestrate_回归_非git子目录不含父仓库文件(t *testing.T) {
	repo := initRepo(t) // 父级是 git，含已提交 f.txt
	sub := filepath.Join(repo, "temp")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	c := Chain{
		Workdir: sub, // 非 git 子目录
		Segments: []Segment{
			{Name: "impl", Profile: "strong", Prompt: "实现"},
			{Name: "review", Profile: "cheap", Prompt: "审查"},
		},
	}
	var reviewPrompt string
	o := NewOrchestrator(testReg())
	o.Auto = true
	o.runSegment = func(spec runSpec, seg Segment) (string, int, error) {
		if seg.Name == "impl" {
			_ = os.WriteFile(filepath.Join(sub, "result.txt"), []byte("成果"), 0o644)
		}
		if seg.Name == "review" {
			reviewPrompt = spec.Prompt
		}
		return "o", 0, nil
	}
	if err := o.Run(c); err != nil {
		t.Fatal(err)
	}
	// review 段应看到子目录里的 result.txt
	if !strings.Contains(reviewPrompt, "result.txt") {
		t.Errorf("review 应含子目录成果 result.txt: %q", reviewPrompt)
	}
	// 但绝不能含父仓库的 f.txt
	if strings.Contains(reviewPrompt, "f.txt") {
		t.Errorf("review 不应含父仓库文件 f.txt: %q", reviewPrompt)
	}
}
