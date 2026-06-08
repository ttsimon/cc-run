// Package tui 提供交互式 Profile 选择。
package tui

import (
	"fmt"

	"ccr/internal/profile"

	"github.com/charmbracelet/huh"
)

// SelectProfile 弹出可过滤的清单，返回用户选中的 Profile。
// 取消选择（Esc/Ctrl-C）时返回 huh.ErrUserAborted。
func SelectProfile(profiles []profile.Profile) (profile.Profile, error) {
	if len(profiles) == 0 {
		return profile.Profile{}, fmt.Errorf("没有可用的 profile")
	}

	opts := make([]huh.Option[int], 0, len(profiles))
	for i, p := range profiles {
		opts = append(opts, huh.NewOption(label(p), i))
	}

	var idx int
	field := huh.NewSelect[int]().
		Title("选择一个 Claude 配置启动").
		Options(opts...).
		Filtering(true).
		Value(&idx)

	if err := huh.NewForm(huh.NewGroup(field)).Run(); err != nil {
		return profile.Profile{}, err
	}
	return profiles[idx], nil
}

// label 渲染一行：名字 [来源] (模型) — 主机名，当前项加 ●。
func label(p profile.Profile) string {
	cur := ""
	if p.IsCurrent {
		cur = " ●"
	}
	model := ""
	if p.Model != "" {
		model = " (" + p.Model + ")"
	}
	host := ""
	if p.BaseURL != "" {
		host = " — " + p.BaseURL
	}
	return fmt.Sprintf("%s%s  [%s]%s%s", p.Name, cur, p.Source, model, host)
}
