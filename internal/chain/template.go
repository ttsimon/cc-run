package chain

import "strings"

// Render 把 prompt 里的 {{prev.output}} 替换为上一段输出（容忍内部空格）。
// 故意只支持固定几个占位符，不引入 text/template，保持简单可预期。
func Render(prompt, prevOutput, input string) string {
	out := prompt
	for _, token := range []string{"{{prev.output}}", "{{ prev.output }}"} {
		out = strings.ReplaceAll(out, token, prevOutput)
	}
	for _, token := range []string{"{{input}}", "{{ input }}"} {
		out = strings.ReplaceAll(out, token, input)
	}
	return out
}
