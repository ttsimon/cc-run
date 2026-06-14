package chain

import (
	"strings"
	"testing"
)

const sampleYAML = `
name: plan-impl-review
isolate: true
segments:
  - name: plan
    profile: cc-switch:strong
    prompt: 把目标拆成 docs/plans/x.md
    allow_tools: [Read, Write]
  - name: implement
    profile: custom:cheap
    prompt: 读 {{prev.output}} 照着实现
  - name: review
    profile: cc-switch:other
    prompt: 审查改动
    review: true
`

func TestParse_合法链(t *testing.T) {
	c, err := Parse([]byte(sampleYAML))
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "plan-impl-review" || !c.Isolate || len(c.Segments) != 3 {
		t.Fatalf("解析不对: %+v", c)
	}
	if c.Segments[0].Profile != "cc-switch:strong" || len(c.Segments[0].AllowTools) != 2 {
		t.Errorf("段0 不对: %+v", c.Segments[0])
	}
	if !c.Segments[2].Review {
		t.Errorf("段2 应为 review")
	}
}

func TestValidate_空段报错(t *testing.T) {
	_, err := Parse([]byte("name: x\nsegments: []\n"))
	if err == nil || !strings.Contains(err.Error(), "段") {
		t.Errorf("空 segments 应报错: %v", err)
	}
}

func TestValidate_段缺profile报错(t *testing.T) {
	_, err := Parse([]byte("name: x\nsegments:\n  - name: a\n    prompt: hi\n"))
	if err == nil {
		t.Error("缺 profile 应报错")
	}
}

func TestUsesInput_含占位返回true(t *testing.T) {
	c := Chain{Segments: []Segment{
		{Prompt: "先看 {{prev.output}}"},
		{Prompt: "实现 {{input}} 这个需求"},
	}}
	if !c.UsesInput() {
		t.Error("有段含 {{input}} 应返回 true")
	}
}

func TestUsesInput_含空格变体返回true(t *testing.T) {
	c := Chain{Segments: []Segment{{Prompt: "做 {{ input }}"}}}
	if !c.UsesInput() {
		t.Error("应容忍 {{ input }} 空格变体")
	}
}

func TestUsesInput_不含返回false(t *testing.T) {
	c := Chain{Segments: []Segment{{Prompt: "纯文本 {{prev.output}}"}}}
	if c.UsesInput() {
		t.Error("不含 {{input}} 应返回 false")
	}
}

func TestParse_AllowPaths字段(t *testing.T) {
	src := `
name: x
segments:
  - name: a
    profile: p
    prompt: hi
    allow_paths:
      - /tmp
      - /var/cache
`
	c, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := c.Segments[0].AllowPaths
	if len(got) != 2 || got[0] != "/tmp" || got[1] != "/var/cache" {
		t.Errorf("allow_paths 解析不对: %v", got)
	}
}
