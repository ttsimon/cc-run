package chain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadVerdict(t *testing.T) {
	dir := t.TempDir()
	vdir := filepath.Join(dir, ".ccr-chain")
	_ = os.MkdirAll(vdir, 0o755)
	_ = os.WriteFile(filepath.Join(vdir, "verdict"), []byte("needs-work\n"), 0o644)
	v := ReadVerdict(dir)
	if v != VerdictNeedsWork {
		t.Errorf("got %v", v)
	}
}

func TestReadVerdict_缺文件为未知(t *testing.T) {
	if ReadVerdict(t.TempDir()) != VerdictUnknown {
		t.Error("缺 verdict 文件应为 Unknown")
	}
}

func TestReviewInstruction_含文件路径(t *testing.T) {
	s := ReviewInstruction()
	if !strings.Contains(s, ".ccr-chain/verdict") || !strings.Contains(s, ".ccr-chain/findings.md") {
		t.Errorf("追加指令应告知写两份文件: %q", s)
	}
}
