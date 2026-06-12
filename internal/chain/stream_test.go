package chain

import "testing"

func TestParseEventLine_工具调用(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"web/index.html"}}]}}`)
	evs := ParseEventLine(line)
	if len(evs) != 1 || evs[0].Kind != EventToolUse {
		t.Fatalf("应解析出一个工具事件, got %+v", evs)
	}
	if evs[0].Tool != "Write" || evs[0].Target != "web/index.html" {
		t.Errorf("工具名/目标错: %+v", evs[0])
	}
}

func TestParseEventLine_Bash取command(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"git add -A\ngit commit"}}]}}`)
	evs := ParseEventLine(line)
	if len(evs) != 1 || evs[0].Target != "git add -A …" {
		t.Errorf("Bash 应取首行 command + 省略号: %+v", evs)
	}
}

func TestParseEventLine_result(t *testing.T) {
	line := []byte(`{"type":"result","subtype":"success","result":"最终答案","usage":{"input_tokens":10,"output_tokens":5}}`)
	evs := ParseEventLine(line)
	if len(evs) != 1 || evs[0].Kind != EventResult {
		t.Fatalf("应解析出 result 事件, got %+v", evs)
	}
	if evs[0].Text != "最终答案" {
		t.Errorf("result 文本错: %q", evs[0].Text)
	}
	if evs[0].Usage == "" {
		t.Errorf("result 应带 usage 摘要")
	}
}

func TestParseEventLine_坏行与未知优雅降级(t *testing.T) {
	if evs := ParseEventLine([]byte(`{不是合法json`)); evs != nil {
		t.Errorf("坏 JSON 应返回 nil, got %+v", evs)
	}
	if evs := ParseEventLine([]byte(`{"type":"system","subtype":"init"}`)); evs != nil {
		t.Errorf("未知/无关 type 应返回 nil, got %+v", evs)
	}
	if evs := ParseEventLine([]byte("   ")); evs != nil {
		t.Errorf("空行应返回 nil")
	}
}

func TestParseEventLine_一条消息多块(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"我来写文件"},{"type":"tool_use","name":"Read","input":{"file_path":"a.go"}}]}}`)
	evs := ParseEventLine(line)
	if len(evs) != 2 {
		t.Fatalf("一条消息两块应出两事件, got %d", len(evs))
	}
	if evs[0].Kind != EventAssistantText || evs[1].Kind != EventToolUse {
		t.Errorf("块顺序/类型错: %+v", evs)
	}
}
