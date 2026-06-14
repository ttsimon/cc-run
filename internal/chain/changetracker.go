package chain

// ChangeTracker 追踪一条链从开始到当前在 workdir 内改动了哪些文件。
// 与 Isolator 的 git/非 git 判定同源，但独立：isolate=false 就地执行时没有
// Isolator，仍需追踪范围。
type ChangeTracker interface {
	// Baseline 在链开始时记录基准（git: HEAD + 已脏文件集；fs: size/mtime 快照）。
	Baseline() error
	// ChangedFiles 返回相对基准、被本链改动的文件（相对 workdir、已排序）。
	ChangedFiles() ([]string, error)
}

// newChangeTracker 按 workdir 是否在 git 工作树内选实现。
func newChangeTracker(workdir string) ChangeTracker {
	if isInsideWorkTree(workdir) {
		return &gitTracker{workdir: workdir}
	}
	return &fsTracker{workdir: workdir}
}

// gitTracker 用 git diff 追踪改动（worktree isolate 与就地 git 模式统一）。
type gitTracker struct {
	workdir   string
	baseSHA   string
	baseDirty map[string]bool
}

func (g *gitTracker) Baseline() error                 { return nil }
func (g *gitTracker) ChangedFiles() ([]string, error) { return nil, nil }

// fsTracker 用 size+mtime 快照追踪改动（非 git 目录）。
type fsTracker struct {
	workdir  string
	snapshot map[string]fileSig
}

type fileSig struct {
	size  int64
	mtime int64
}

func (f *fsTracker) Baseline() error                 { return nil }
func (f *fsTracker) ChangedFiles() ([]string, error) { return nil, nil }
