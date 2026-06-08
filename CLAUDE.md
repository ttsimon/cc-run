# CLAUDE.md

给 Claude Code 的项目上下文。换机器 / 新会话 / 隔久回来，先读这里。

## 这是什么

`ccr` —— 跨平台命令行工具：把选定 provider 的环境变量注入**当前终端**再拉起 `claude`，从而可同时多开、各用不同后端，互不干扰，且**不改全局配置**。

与 cc-switch 的区别：cc-switch 切换**全局**当前 provider（同时只有一个生效）；ccr 是**按终端会话**注入 env，多开 = 开多个终端各跑一次 `ccr <名字>`。

## 架构（三层，职责单一）

```
Sources(来源)            Registry(合并)         Launcher(启动)
 ├ ccswitch  只读 SQLite ┐
 │  ~/.cc-switch/*.db     ├─ 合并+标注来源+按名解析 ─ 组装 env + 拉起 claude(透传退出码)
 └ customdir 读 *.json   ┘
```

- 两个配置来源：cc-switch 库（只读）+ 自定义目录 `~/.ccr/profiles/*.json`，合并为一个列表、标注来源、重名用 `来源:名字` 消歧。
- 纯逻辑（解析/合并/env 组装/参数）全部单元测试；拉起子进程用 Go helper-process 模式集成测试；TUI/editor 手动验证。

## 代码布局

```
cmd/ccr/            入口（go install 装出的命令名 = ccr）
internal/profile/   Profile 类型 + token 打码
internal/source/    parse + ccswitch(SQLite) + customdir(JSON)
internal/config/    路径解析 env > ~/.ccr/config.json > 默认
internal/registry/  合并与按名解析
internal/launcher/  ComposeEnv / ClaudeArgs / Run
internal/cli/       参数分发 ls/show/edit/<name>/交互
internal/tui/       huh fuzzy 选择器
tools/commitlint/   commit-msg 校验（被 lefthook 调用）
docs/superpowers/   设计 spec 与实现计划
```

模块路径 `github.com/ttsimon/cc-run`；技术栈 Go（CGO_ENABLED=0）、`modernc.org/sqlite`（免 cgo）、`charmbracelet/huh`。

## 约定（重要）

- **提交信息**：Conventional Commits（`feat:`/`fix:`/`docs:`/`build:`/`ci:`/`chore:`/`refactor:`/`test:`/`security:`），commit-msg 钩子强制。只有 `feat:`/`fix:` 进发布说明。
- **提交前**：`task check`（fmt + vet + lint + test）。
- **密钥安全**：绝不提交真实 token/key，哪怕在测试或文档里。假值一律带 `FAKE` 标记（如 `sk-FAKE...`），已在 `.gitleaks.toml` 放行；真实随机密钥会被 gitleaks 拦。
- **不手写 CHANGELOG**：发布说明由 GoReleaser 从 commit 自动生成，Releases 页即变更日志。

## 常用命令

```
task            列出全部
task build      构建
task test       测试
task check      提交前全套
task lint       golangci-lint
task hooks      启用 git 钩子（每台机器克隆后跑一次）
task snapshot   本地试打包（不发布）
```

## 发布

打 tag 自动发布（全平台二进制 + Scoop/Homebrew + 自动 release notes）：
```
git tag v0.1.1 && git push origin v0.1.1
```
已发布的 tag 不可移动（Go module proxy 永久缓存 tag→commit）。详见 `RELEASING.md`、贡献流程见 `CONTRIBUTING.md`。

## 跨设备

本仓库即唯一真相：换电脑 `git pull` 即可，进度看 commits + 计划文档勾选框 + GitHub。
master 受 Ruleset 保护（要求 CI 绿 + 走 PR），所以本地改动要开分支 → PR。
