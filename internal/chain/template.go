package chain

import "strings"

// Render 把 prompt 里的 {{prev.output}}（容忍内部空格）替换为上一段输出。
// 故意只支持这一个占位符，不引入 text/template，保持简单可预期。
func Render(prompt, prevOutput string) string {
	out := prompt
	for _, token := range []string{"{{prev.output}}", "{{ prev.output }}"} {
		out = strings.ReplaceAll(out, token, prevOutput)
	}
	return out
}
