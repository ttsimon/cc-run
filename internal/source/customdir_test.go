package source

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCustomDir_LoadsJSONFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "deepseek.json", `{"env":{"ANTHROPIC_BASE_URL":"https://api.deepseek.com/anthropic"}}`)
	writeFile(t, dir, "notes.txt", `ignore me`)

	src := NewCustomDir(dir)
	if !src.Available() {
		t.Fatal("目录存在时 Available 应为 true")
	}
	ps, err := src.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 {
		t.Fatalf("应只加载 1 个 .json, got %d", len(ps))
	}
	if ps[0].Name != "deepseek" {
		t.Errorf("Name 应取文件名去扩展名, got %q", ps[0].Name)
	}
	if ps[0].Source != "custom" {
		t.Errorf("Source = %q", ps[0].Source)
	}
}

func TestCustomDir_坏文件跳过其余照常(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good.json", `{"env":{}}`)
	writeFile(t, dir, "bad.json", `{not json`)

	src := NewCustomDir(dir)
	src.Warnf = func(string, ...any) {} // 静默测试输出
	ps, err := src.Load()
	if err != nil {
		t.Fatalf("坏文件不应整体报错: %v", err)
	}
	if len(ps) != 1 || ps[0].Name != "good" {
		t.Fatalf("应只加载 good, got %+v", ps)
	}
}

func TestCustomDir_目录不存在(t *testing.T) {
	src := NewCustomDir(filepath.Join(t.TempDir(), "nope"))
	if src.Available() {
		t.Error("不存在的目录 Available 应为 false")
	}
}
