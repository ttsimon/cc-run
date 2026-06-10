package chain

import (
	"os"
	"path/filepath"
	"strings"
)

// Verdict 是审查段的判定。
type Verdict int

const (
	VerdictUnknown Verdict = iota
	VerdictPass
	VerdictNeedsWork
)

// ReadVerdict 读 workdir/.ccr-chain/verdict；缺文件或无法识别为 Unknown。
func ReadVerdict(workdir string) Verdict {
	raw, err := os.ReadFile(filepath.Join(workdir, ".ccr-chain", "verdict"))
	if err != nil {
		return VerdictUnknown
	}
	switch strings.TrimSpace(string(raw)) {
	case "pass":
		return VerdictPass
	case "needs-work":
		return VerdictNeedsWork
	default:
		return VerdictUnknown
	}
}

// ReviewInstruction 是追加到审查段 prompt 末尾的固定指令。
func ReviewInstruction() string {
	return "\n\n---\n审查完成后，必须：1) 把问题清单写到 .ccr-chain/findings.md；" +
		"2) 在 .ccr-chain/verdict 写单独一行，内容为 pass 或 needs-work。"
}
