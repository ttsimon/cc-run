package chain

import "testing"

func TestNewIsolator_git目录选worktree(t *testing.T) {
	repo := initRepo(t)
	iso, err := newIsolator(repo, "t")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := iso.(*worktreeIsolator); !ok {
		t.Errorf("git 目录应选 worktreeIsolator, got %T", iso)
	}
}

func TestNewIsolator_非git目录选copydir(t *testing.T) {
	dir := t.TempDir()
	iso, err := newIsolator(dir, "t")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := iso.(*copydirIsolator); !ok {
		t.Errorf("非 git 目录应选 copydirIsolator, got %T", iso)
	}
}
