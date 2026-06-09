// Package overlay 读写 ccr 旁挂在 ~/.ccr 的元数据：
// overlay.json 是用户意图（别名表 + 默认 profile），state.json 是运行痕迹（上次用的）。
// cc-switch 库只读，这些写不回去，故另存一份。
package overlay

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// userHomeDir 便于测试替换。
var userHomeDir = os.UserHomeDir

// Overlay 是用户显式设的元数据。
type Overlay struct {
	Aliases map[string]string `json:"aliases"` // 别名 -> profile 查询名（可为 source:name）
	Default string            `json:"default"` // 默认 profile 查询名
}

// State 是运行时自动记录的状态。
type State struct {
	Last string `json:"last"` // 上次成功拉起的 profile（source:name）
}

func ccrDir() string {
	home, err := userHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".ccr")
}

func overlayPath() string { return filepath.Join(ccrDir(), "overlay.json") }
func statePath() string   { return filepath.Join(ccrDir(), "state.json") }

// LoadOverlay 读 overlay.json；缺文件或坏文件回退到空 Overlay（Aliases 始终非 nil）。
func LoadOverlay() Overlay {
	o := Overlay{Aliases: map[string]string{}}
	if raw, err := os.ReadFile(overlayPath()); err == nil {
		_ = json.Unmarshal(raw, &o) // 坏文件忽略
	}
	if o.Aliases == nil {
		o.Aliases = map[string]string{}
	}
	return o
}

// SaveOverlay 写 overlay.json（0600，目录 0700）。
func SaveOverlay(o Overlay) error {
	return writeJSON(overlayPath(), o)
}

// LoadState 读 state.json；缺/坏回退空 State。
func LoadState() State {
	var s State
	if raw, err := os.ReadFile(statePath()); err == nil {
		_ = json.Unmarshal(raw, &s)
	}
	return s
}

// SaveState 写 state.json。
func SaveState(s State) error {
	return writeJSON(statePath(), s)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
