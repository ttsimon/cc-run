package source

import (
	"database/sql"
	"fmt"
	"os"

	"ccr/internal/profile"

	_ "modernc.org/sqlite"
)

// CCSwitch 以只读方式从 cc-switch 的 SQLite 库读取 claude provider。
type CCSwitch struct {
	Path string
}

// NewCCSwitch 构造一个 cc-switch 来源。
func NewCCSwitch(path string) *CCSwitch {
	return &CCSwitch{Path: path}
}

// Available 报告库文件是否存在。
func (c *CCSwitch) Available() bool {
	info, err := os.Stat(c.Path)
	return err == nil && !info.IsDir()
}

// Load 只读打开库，读取 app_type='claude' 的 provider。
func (c *CCSwitch) Load() ([]profile.Profile, error) {
	dsn := "file:" + c.Path + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 cc-switch 库失败: %w", err)
	}
	defer db.Close()
	// 只读期间避免长时间等待写锁。
	_, _ = db.Exec("PRAGMA busy_timeout=2000")

	rows, err := db.Query(
		`SELECT name, settings_config, COALESCE(is_current,0) FROM providers WHERE app_type='claude'`,
	)
	if err != nil {
		return nil, fmt.Errorf("查询 providers 失败: %w", err)
	}
	defer rows.Close()

	var out []profile.Profile
	for rows.Next() {
		var name, cfg string
		var isCur int
		if err := rows.Scan(&name, &cfg, &isCur); err != nil {
			return nil, err
		}
		p, err := ParseSettingsConfig(name, profile.SourceCCSwitch, cfg)
		if err != nil {
			// 单条坏数据不致命：跳过。
			continue
		}
		p.IsCurrent = isCur != 0
		out = append(out, p)
	}
	return out, rows.Err()
}
