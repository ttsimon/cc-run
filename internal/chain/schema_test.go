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
