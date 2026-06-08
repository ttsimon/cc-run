package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ttsimon/cc-run/internal/profile"
)

// CustomDir 从一个目录读取 *.json，每个文件一个 Profile。
type CustomDir struct {
	Dir string
	// Warnf 处理坏文件告警，默认写 stderr；测试可替换。
	Warnf func(format string, a ...any)
}

// NewCustomDir 构造一个自定义目录来源。
func NewCustomDir(dir string) *CustomDir {
	return &CustomDir{
		Dir:   dir,
		Warnf: func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) },
	}
}

// Available 报告目录是否存在。
func (c *CustomDir) Available() bool {
	info, err := os.Stat(c.Dir)
	return err == nil && info.IsDir()
}

// Load 读取目录下所有 *.json；坏文件告警并跳过。
func (c *CustomDir) Load() ([]profile.Profile, error) {
	matches, err := filepath.Glob(filepath.Join(c.Dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var out []profile.Profile
	for _, path := range matches {
		raw, err := os.ReadFile(path)
		if err != nil {
			c.Warnf("跳过 %s: %v", path, err)
			continue
		}
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		p, err := ParseSettingsConfig(name, profile.SourceCustom, string(raw))
		if err != nil {
			c.Warnf("跳过 %s: %v", path, err)
			continue
		}
		out = append(out, p)
	}
	return out, nil
}
