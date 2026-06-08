// Package registry 合并多来源的 Profile 并按名解析。
package registry

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ttsimon/cc-run/internal/profile"
)

// Registry 持有合并后的 Profile 列表。
type Registry struct {
	profiles []profile.Profile
}

// New 用一组 Profile 构造 Registry。
func New(profiles []profile.Profile) *Registry {
	return &Registry{profiles: profiles}
}

// List 返回全部 Profile（按 来源、名字 稳定排序）。
func (r *Registry) List() []profile.Profile {
	out := make([]profile.Profile, len(r.profiles))
	copy(out, r.profiles)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Resolve 按 query 找唯一 Profile。query 可为 "name" 或 "source:name"。
func (r *Registry) Resolve(query string) (profile.Profile, error) {
	wantSource, name := splitQuery(query)

	var matches []profile.Profile
	for _, p := range r.profiles {
		if p.Name != name {
			continue
		}
		if wantSource != "" && string(p.Source) != wantSource {
			continue
		}
		matches = append(matches, p)
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return profile.Profile{}, r.notFoundErr(name)
	default:
		var qualified []string
		for _, p := range matches {
			qualified = append(qualified, fmt.Sprintf("%s:%s", p.Source, p.Name))
		}
		return profile.Profile{}, fmt.Errorf(
			"名字 %q 有多个来源，请用限定名指定其一：%s",
			name, strings.Join(qualified, " 、 "),
		)
	}
}

// splitQuery 拆出可选的 "source:" 前缀。
func splitQuery(q string) (source, name string) {
	if i := strings.IndexByte(q, ':'); i > 0 {
		prefix := q[:i]
		if prefix == string(profile.SourceCCSwitch) || prefix == string(profile.SourceCustom) {
			return prefix, q[i+1:]
		}
	}
	return "", q
}

// notFoundErr 构造带建议的未命中错误。
func (r *Registry) notFoundErr(name string) error {
	var suggestions []string
	lower := strings.ToLower(name)
	for _, p := range r.profiles {
		if strings.Contains(strings.ToLower(p.Name), lower) {
			suggestions = append(suggestions, p.Name)
		}
	}
	if len(suggestions) == 0 {
		return fmt.Errorf("找不到名为 %q 的配置；用 `ccr ls` 查看全部", name)
	}
	return fmt.Errorf("找不到 %q，你是不是指：%s", name, strings.Join(suggestions, " 、 "))
}
