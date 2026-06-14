package chain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewChangeTracker_git目录选gitTracker(t *testing.T) {
	repo := initRepo(t)
	tr := newChangeTracker(repo)
	if _, ok := tr.(*gitTracker); !ok {
		t.Errorf("git 工作树应选 gitTracker, got %T", tr)
	}
}

func TestNewChangeTracker_非git目录选fsTracker(t *testing.T) {
	dir := t.TempDir()
	tr := newChangeTracker(dir)
	if _, ok := tr.(*fsTracker); !ok {
		t.Errorf("非 git 目录应选 fsTracker, got %T", tr)
	}
}

func writeFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGitTracker_捕获新增与修改(t *testing.T) {
	repo := initRepo(t) // 已含已提交的 f.txt
	g := &gitTracker{workdir: repo}
	if err := g.Baseline(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, "new.txt", "新文件")
	writeFile(t, repo, "f.txt", "改了内容")
	got, err := g.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"f.txt", "new.txt"}
	if !equalStrs(got, want) {
		t.Errorf("ChangedFiles = %v, want %v", got, want)
	}
}

func TestGitTracker_扣除链前已脏文件(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "dirty.txt", "链开始前就脏")
	g := &gitTracker{workdir: repo}
	if err := g.Baseline(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, "real.txt", "本链改动")
	got, err := g.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if !equalStrs(got, []string{"real.txt"}) {
		t.Errorf("应扣除链前已脏的 dirty.txt, got %v", got)
	}
}

func TestGitTracker_已提交改动仍计入(t *testing.T) {
	repo := initRepo(t)
	g := &gitTracker{workdir: repo}
	if err := g.Baseline(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, "sealed.txt", "封存的成果")
	if out, err := gitIn(repo, "add", "-A"); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	if out, err := gitIn(repo, "commit", "-m", "seal"); err != nil {
		t.Fatalf("git commit: %v %s", err, out)
	}
	got, err := g.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if !equalStrs(got, []string{"sealed.txt"}) {
		t.Errorf("diff baseSHA 应覆盖已提交改动, got %v", got)
	}
}

// equalStrs 顺序敏感比较——ChangedFiles 契约承诺输出已排序，这里不排序，
// 以便测试能锁住该契约（want 值已按排序顺序写好）。
func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
