package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestApply_非TTY不加ANSI(t *testing.T) {
	got := Apply(false, StyleOK, "ok")
	if got != "ok" {
		t.Errorf("非 TTY 应原样返回, got %q", got)
	}
	if strings.Contains(got, "\x1b[") {
		t.Errorf("非 TTY 不应含 ANSI: %q", got)
	}
}

func TestWriterIsTTY_非文件为false(t *testing.T) {
	if WriterIsTTY(&bytes.Buffer{}) {
		t.Error("bytes.Buffer 不是 TTY")
	}
}

func TestIcons_非空(t *testing.T) {
	for _, s := range []string{IconTool, IconOK, IconRun, IconPause} {
		if s == "" {
			t.Error("符号不应为空")
		}
	}
}
