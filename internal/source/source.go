package source

import "ccr/internal/profile"

// ProfileSource 是一个 Profile 来源。
type ProfileSource interface {
	// Available 报告该来源是否存在（如库文件/目录是否在）。
	Available() bool
	// Load 解析并返回该来源的所有 Profile。
	Load() ([]profile.Profile, error)
}

// LoadAll 加载所有 Available 的来源，汇总 Profile 与各自的错误（错误不致命）。
func LoadAll(srcs ...ProfileSource) ([]profile.Profile, []error) {
	var all []profile.Profile
	var errs []error
	for _, s := range srcs {
		if !s.Available() {
			continue
		}
		ps, err := s.Load()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		all = append(all, ps...)
	}
	return all, errs
}
