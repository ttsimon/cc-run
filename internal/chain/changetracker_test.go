package chain

import "testing"

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
