package registry

import (
	"strings"
	"testing"

	"github.com/ttsimon/cc-run/internal/profile"
)

func sample() []profile.Profile {
	return []profile.Profile{
		{Name: "DeepSeek", Source: profile.SourceCCSwitch},
		{Name: "火山", Source: profile.SourceCCSwitch},
		{Name: "DeepSeek", Source: profile.SourceCustom}, // 与 cc-switch 同名
		{Name: "my-local", Source: profile.SourceCustom},
	}
}

func TestResolve_唯一匹配(t *testing.T) {
	r := New(sample())
	p, err := r.Resolve("火山")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "火山" {
		t.Errorf("got %q", p.Name)
	}
}

func TestResolve_重名歧义报错(t *testing.T) {
	r := New(sample())
	_, err := r.Resolve("DeepSeek")
	if err == nil {
		t.Fatal("重名应报歧义错误")
	}
	if !strings.Contains(err.Error(), "cc-switch:DeepSeek") ||
		!strings.Contains(err.Error(), "custom:DeepSeek") {
		t.Errorf("歧义错误应给出限定名: %v", err)
	}
}

func TestResolve_限定名消歧(t *testing.T) {
	r := New(sample())
	p, err := r.Resolve("custom:DeepSeek")
	if err != nil {
		t.Fatal(err)
	}
	if p.Source != profile.SourceCustom {
		t.Errorf("应取 custom, got %q", p.Source)
	}
}

func TestResolve_未命中给建议(t *testing.T) {
	r := New(sample())
	_, err := r.Resolve("deep")
	if err == nil {
		t.Fatal("未精确命中应报错")
	}
	if !strings.Contains(err.Error(), "DeepSeek") {
		t.Errorf("应建议含子串的名字: %v", err)
	}
}

func TestList_返回全部(t *testing.T) {
	r := New(sample())
	if len(r.List()) != 4 {
		t.Errorf("List 应返回全部 4 个")
	}
}
