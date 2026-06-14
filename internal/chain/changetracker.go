package chain

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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

func (g *gitTracker) Baseline() error {
	out, err := gitIn(g.workdir, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("读 baseSHA 失败: %v %s", err, out)
	}
	g.baseSHA = strings.TrimSpace(string(out))
	g.baseDirty = map[string]bool{}
	for _, f := range g.statusFiles() {
		g.baseDirty[f] = true
	}
	return nil
}

func (g *gitTracker) ChangedFiles() ([]string, error) {
	set := map[string]bool{}
	if out, err := gitIn(g.workdir, "diff", "--name-only", g.baseSHA); err == nil {
		for _, f := range splitLines(string(out)) {
			set[f] = true
		}
	}
	if out, err := gitIn(g.workdir, "ls-files", "--others", "--exclude-standard"); err == nil {
		for _, f := range splitLines(string(out)) {
			set[f] = true
		}
	}
	var files []string
	for f := range set {
		if !g.baseDirty[f] {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files, nil
}

// statusFiles 返回 git status --porcelain 列出的文件名（含 untracked）。
func (g *gitTracker) statusFiles() []string {
	out, err := gitIn(g.workdir, "status", "--porcelain")
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range splitLines(string(out)) {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		if len(line) > 3 {
			s = strings.TrimSpace(line[3:])
		}
		if idx := strings.Index(s, " -> "); idx >= 0 {
			s = s[idx+len(" -> "):]
		}
		files = append(files, s)
	}
	return files
}

// splitLines 按行切并丢空行（git --name-only 等输出尾随空行）。
func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// fsTracker 用 size+mtime 快照追踪改动（非 git 目录）。
type fsTracker struct {
	workdir  string
	snapshot map[string]fileSig
}

type fileSig struct {
	size  int64
	mtime int64
}

func (f *fsTracker) Baseline() error {
	snap, err := f.scan()
	if err != nil {
		return err
	}
	f.snapshot = snap
	return nil
}

func (f *fsTracker) ChangedFiles() ([]string, error) {
	cur, err := f.scan()
	if err != nil {
		return nil, err
	}
	var files []string
	for rel, sig := range cur {
		old, ok := f.snapshot[rel]
		if !ok || old != sig {
			files = append(files, rel)
		}
	}
	sort.Strings(files)
	return files, nil
}

func (f *fsTracker) scan() (map[string]fileSig, error) {
	out := map[string]fileSig{}
	err := filepath.WalkDir(f.workdir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(f.workdir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		top := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
		if top == ".git" || top == ".ccr-chain" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out[filepath.ToSlash(rel)] = fileSig{size: info.Size(), mtime: info.ModTime().UnixNano()}
		return nil
	})
	return out, err
}
