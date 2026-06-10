// Package doctor 探测某 Profile 的后端是否可达，用于跑 chain 前的体检。
package doctor

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ttsimon/cc-run/internal/profile"
)

// Result 是一次体检结果。
type Result struct {
	Name   string
	OK     bool
	Detail string // 失败原因或 "HTTP 200" 等
}

// httpClient 便于测试替换超时。
var httpClient = &http.Client{Timeout: 5 * time.Second}

// Check 对 p 的 ANTHROPIC_BASE_URL 发一个 GET，状态码 <500 视为「后端在」。
// 401/403 说明端点活着只是鉴权——对「可达性」而言算通。
func Check(p profile.Profile) Result {
	base := p.Env["ANTHROPIC_BASE_URL"]
	if base == "" {
		return Result{Name: p.Name, OK: false, Detail: "无 ANTHROPIC_BASE_URL"}
	}
	resp, err := httpClient.Get(base)
	if err != nil {
		return Result{Name: p.Name, OK: false, Detail: err.Error()}
	}
	defer resp.Body.Close()
	ok := resp.StatusCode < 500
	return Result{Name: p.Name, OK: ok, Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}
