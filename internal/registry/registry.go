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
	matches := r.exactMatches(query)
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		_, name := splitQuery(query)
		return profile.Profile{}, r.notFoundErr(name)
	default:
		var qualified []string
		for _, p := range matches {
			qualified = append(qualified, fmt.Sprintf("%s:%s", p.Source, p.Name))
		}
		return profile.Profile{}, fmt.Errorf(
			"名字 %q 有多个来源，请用限定名指定其一：%s",
			query, strings.Join(qualified, " 、 "),
		)
	}
}

// exactMatches 返回精确（含 source:name 限定）命中的 Profile 列表（0/1/多）。
func (r *Registry) exactMatches(query string) []profile.Profile {
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
	return matches
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

// LookupResult 表示一次按名解析的结果：
// 命中唯一 → Profile 有值；模糊多命中 → Candidates 有值，交给 TUI 选择。
type LookupResult struct {
	Profile    profile.Profile
	Candidates []profile.Profile
}

// Lookup 按 query 解析，依次尝试：精确名/限定名 → 别名（精确）→ 模糊子串。
// aliases: 别名 -> 查询名，可为 nil。
func (r *Registry) Lookup(query string, aliases map[string]string) (LookupResult, error) {
	// 1) 精确名 / source:name
	switch m := r.exactMatches(query); len(m) {
	case 1:
		return LookupResult{Profile: m[0]}, nil
	case 0:
		// 落到别名 / 模糊
	default:
		var qualified []string
		for _, p := range m {
			qualified = append(qualified, fmt.Sprintf("%s:%s", p.Source, p.Name))
		}
		return LookupResult{}, fmt.Errorf(
			"名字 %q 有多个来源，请用限定名指定其一：%s",
			query, strings.Join(qualified, " 、 "),
		)
	}

	// 2) 别名（精确匹配别名键，目标再做精确解析）
	if target, ok := aliases[query]; ok {
		if m := r.exactMatches(target); len(m) == 1 {
			return LookupResult{Profile: m[0]}, nil
		}
		return LookupResult{}, fmt.Errorf("别名 %q 指向 %q，但它无法唯一解析", query, target)
	}

	// 3) 模糊：名字含 query（不分大小写）
	lower := strings.ToLower(query)
	var cand []profile.Profile
	for _, p := range r.profiles {
		if strings.Contains(strings.ToLower(p.Name), lower) {
			cand = append(cand, p)
		}
	}
	switch len(cand) {
	case 1:
		return LookupResult{Profile: cand[0]}, nil
	case 0:
		return LookupResult{}, r.notFoundErr(query)
	default:
		return LookupResult{Candidates: cand}, nil
	}
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
