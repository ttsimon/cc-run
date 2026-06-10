package chain

import (
	"strings"
	"testing"
)

func TestTemplate_存在且可解析(t *testing.T) {
	raw, ok := Template("plan-impl-review")
	if !ok {
		t.Fatal("内置模板应存在")
	}
	if _, err := Parse([]byte(raw)); err != nil {
		t.Fatalf("内置模板应能解析: %v", err)
	}
	if !strings.Contains(raw, "{{prev.output}}") {
		t.Error("模板应演示 prev.output 交棒")
	}
}

func TestTemplate_未知名返回false(t *testing.T) {
	if _, ok := Template("nope"); ok {
		t.Error("未知模板应返回 false")
	}
}
