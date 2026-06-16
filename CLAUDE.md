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
- **旁挂元数据层** `~/.ccr/overlay.json`（别名 + 默认）+ `~/.ccr/state.json`（上次用的）：cc-switch 库只读，别名/默认/上次写不回去，故另存一份；按名解析时叠加——特殊记号 `-`（上次）/ `.`（默认）、别名、模糊子串命中（唯一直启、多命中弹过滤选择器）。
- **chain（v0.3）**：多后端 agent 流水线——无头 `claude -p` 分段、共享工作目录 + `{{prev.output}}` 交棒 + `--input`/`{{input}}` 注入需求、改动文件集软提示注入下游、默认段间放行 / `--auto`（含 `-q`/`-v` 详简）、审查判定 fail-closed 自动合并/保留、PreToolUse 钩子三道闸（命令黑名单 + cd 上跳 + 路径围栏）、可插拔隔离（worktree/copydir，`isolate` 默认 false）+ 三态成果交回。详见 `docs/superpowers/specs/2026-06-09-ccr-chain-design.md`。
- 纯逻辑（解析/合并/env 组装/参数）全部单元测试；拉起子进程用 Go helper-process 模式集成测试；TUI/editor 手动验证。

## 代码布局

```
cmd/ccr/            入口（go install 装出的命令名 = ccr）
internal/profile/   Profile 类型 + token 打码
internal/source/    parse + ccswitch(SQLite) + customdir(JSON)
internal/config/    路径解析 env > ~/.ccr/config.json > 默认
internal/overlay/   旁挂元数据：overlay.json(别名/默认) + state.json(上次用的)
internal/registry/  合并 + 按名解析（精确 / 别名 / 模糊命中）
internal/launcher/  ComposeEnv / ClaudeArgs / Run
internal/completion/ 各 shell 补全脚本(bash/zsh/powershell) + 一键装卸(幂等)
internal/doctor/    后端可达性体检（ccr doctor）
internal/chain/     多后端流水线：yaml 解析/模板/段执行/编排/放行/审查判定/黑名单钩子/worktree
internal/cli/       参数分发 ls/show/edit/alias/unalias/default/completion/-/./<name>/交互
internal/tui/       huh fuzzy 选择器
internal/ui/        共享终端样式层（lipgloss 调色板/符号 + TTY 降级）
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
