package chain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// EventKind 是从 stream-json 抽出的内部事件类型（与具体 schema 解耦）。
type EventKind int

const (
	EventOther EventKind = iota
	EventToolUse
	EventAssistantText
	EventResult
)

// Event 是段无关的渲染单元。
type Event struct {
	Kind   EventKind
	Tool   string // EventToolUse：工具名
	Target string // EventToolUse：主要参数摘要（文件路径/命令首行）
	Text   string // EventAssistantText / EventResult：文本
	Usage  string // EventResult：token 摘要
}

// rawEvent 对应 claude -p --output-format stream-json --verbose 的一行。
//
// ⚠️ 集成边界：字段名按当前 Claude Code stream-json 写。实现时跑一次真
// `claude -p --output-format stream-json --verbose` 校准；schema 漂移只改本文件。
type rawEvent struct {
	Type    string `json:"type"`
	Result  string `json:"result"`
	Message struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	} `json:"message"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ParseEventLine 把一行 stream-json 解析为零或多个内部事件。
// 防脆：空行/坏 JSON/未知 type 一律返回 nil，绝不报错中断整段。
func ParseEventLine(line []byte) []Event {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return nil
	}
	var raw rawEvent
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil
	}
	switch raw.Type {
	case "assistant":
		var evs []Event
		for _, b := range raw.Message.Content {
			switch b.Type {
			case "text":
				if strings.TrimSpace(b.Text) != "" {
					evs = append(evs, Event{Kind: EventAssistantText, Text: b.Text})
				}
			case "tool_use":
				evs = append(evs, Event{
					Kind:   EventToolUse,
					Tool:   b.Name,
					Target: toolTarget(b.Input),
				})
			}
		}
		return evs
	case "result":
		return []Event{{
			Kind:  EventResult,
			Text:  raw.Result,
			Usage: fmt.Sprintf("in %d / out %d", raw.Usage.InputTokens, raw.Usage.OutputTokens),
		}}
	default:
		return nil // system / user(tool_result) / 未知 → 不渲染
	}
}

// toolTarget 从工具 input 抽一个简短目标；抽不到返回空。
func toolTarget(input json.RawMessage) string {
	var m map[string]any
	if json.Unmarshal(input, &m) != nil {
		return ""
	}
	for _, k := range []string{"file_path", "path", "command", "pattern", "url"} {
		if v, ok := m[k].(string); ok && v != "" {
			return firstLine(v)
		}
	}
	return ""
}

// firstLine 取首行；有多行时加省略标记。
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i]) + " …"
	}
	return s
}
