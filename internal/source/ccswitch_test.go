package source

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// newFixtureDB 在临时目录建一个含 providers 表的可写库并插入若干行。
func newFixtureDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cc-switch.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE providers (
		id TEXT, app_type TEXT, name TEXT, settings_config TEXT, is_current INTEGER DEFAULT 0
	)`); err != nil {
		t.Fatal(err)
	}
	rows := []struct {
		app, name, cfg string
		cur            int
	}{
		{"claude", "default", `{"model":"opus"}`, 1},
		{"claude", "DeepSeek", `{"env":{"ANTHROPIC_BASE_URL":"https://api.deepseek.com/anthropic"}}`, 0},
		{"codex", "OpenAI", `{"auth":{}}`, 0}, // 非 claude，应被过滤
	}
	for i, r := range rows {
		if _, err := db.Exec(
			`INSERT INTO providers(id,app_type,name,settings_config,is_current) VALUES(?,?,?,?,?)`,
			i, r.app, r.name, r.cfg, r.cur,
		); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestCCSwitch_只加载claude且解析正确(t *testing.T) {
	path := newFixtureDB(t)
	src := NewCCSwitch(path)
	if !src.Available() {
		t.Fatal("库存在时 Available 应为 true")
	}
	ps, err := src.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Fatalf("应只加载 2 个 claude provider, got %d (%+v)", len(ps), ps)
	}
	byName := map[string]bool{}
	for _, p := range ps {
		byName[p.Name] = p.IsCurrent
		if p.Source != "cc-switch" {
			t.Errorf("Source = %q", p.Source)
		}
	}
	if !byName["default"] {
		t.Error("default 的 IsCurrent 应为 true")
	}
	if byName["DeepSeek"] {
		t.Error("DeepSeek 的 IsCurrent 应为 false")
	}
}

func TestCCSwitch_库不存在(t *testing.T) {
	src := NewCCSwitch(filepath.Join(t.TempDir(), "nope.db"))
	if src.Available() {
		t.Error("库不存在时 Available 应为 false")
	}
}
