// commitlint 校验 commit message 是否符合 Conventional Commits。
// 用法： go run ./tools/commitlint <commit-msg-file>
// 由 lefthook 的 commit-msg 钩子调用。
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// 允许的类型；与 .goreleaser.yaml 的 changelog 分组/过滤保持一致。
var pattern = regexp.MustCompile(`^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert|security)(\([^)]+\))?!?: .+`)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: commitlint <commit-msg-file>")
		os.Exit(2)
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "读取 commit message 失败:", err)
		os.Exit(2)
	}

	// 取第一行非空内容作为标题。
	var subject string
	for _, line := range strings.Split(string(raw), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		subject = s
		break
	}

	// merge / revert / fixup 等自动提交放行。
	if strings.HasPrefix(subject, "Merge ") ||
		strings.HasPrefix(subject, "Revert ") ||
		strings.HasPrefix(subject, "fixup!") ||
		strings.HasPrefix(subject, "squash!") {
		return
	}

	if !pattern.MatchString(subject) {
		fmt.Fprintf(os.Stderr, `✗ commit message 不符合 Conventional Commits：
  %q

格式： <类型>[(可选范围)][!]: <描述>
类型： feat fix docs style refactor perf test build ci chore revert security
示例： feat(cli): 加 --json 输出
      fix: 修复空配置目录崩溃
`, subject)
		os.Exit(1)
	}
}
