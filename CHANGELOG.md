# Changelog

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 与 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [0.1.0] - 2026-06-08

### Added

- 首个版本：`ccr` 用选定 provider 的环境变量启动 `claude`，可同时多开、各用不同后端，互不干扰，且不改全局配置。
- 两个配置来源：cc-switch SQLite 库（只读）+ 自定义目录 `~/.ccr/profiles/*.json`，合并为一个列表并标注来源。
- 命令：交互式 fuzzy 选择、按名直启并透传参数、`ls`、`show`（token 默认打码，`--reveal` 显示完整）、`edit`（用 `$EDITOR` 编辑/新建自定义配置）、`--version`。
- 重名消歧：`来源:名字`（如 `cc-switch:DeepSeek`）。
- 跨平台单文件二进制（Windows / macOS / Linux），纯 Go 实现（`modernc.org/sqlite` 免 cgo）。
- 多渠道分发：GitHub Release、`go install`、Scoop、Homebrew cask（GoReleaser + GitHub Actions）。

[Unreleased]: https://github.com/ttsimon/cc-run/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/ttsimon/cc-run/releases/tag/v0.1.0
