# ccr

[![CI](https://github.com/ttsimon/cc-run/actions/workflows/ci.yml/badge.svg)](https://github.com/ttsimon/cc-run/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ttsimon/cc-run)](https://github.com/ttsimon/cc-run/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/ttsimon/cc-run.svg)](https://pkg.go.dev/github.com/ttsimon/cc-run)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

用选定 provider 的环境变量启动 `claude`，从而可同时多开、各用不同后端，互不干扰，且不改全局配置。

`cc-switch` 切换的是**全局**当前 provider（同一时刻只有一个）。`ccr` 则给**当前这个终端**注入某个 provider 的环境变量再拉起 `claude`——你想多开就开多个终端 tab，每个跑一次 `ccr <名字>`，各用各的后端。

## 安装

任选一种：

**1. 预编译二进制（任何系统，无需 Go）**
到 [Releases](https://github.com/ttsimon/cc-run/releases) 下载对应系统的压缩包，解压后把 `ccr`（Windows 为 `ccr.exe`）放进 PATH。

**2. `go install`（有 Go 环境）**
```
go install github.com/ttsimon/cc-run/cmd/ccr@latest
```

**3. Scoop（Windows）**
```
scoop bucket add ttsimon https://github.com/ttsimon/scoop-bucket
scoop install ccr
```

**4. Homebrew（macOS / Linux）**
```
brew install ttsimon/tap/ccr
```

**从源码自行构建：**
```
go build -o ccr ./cmd/ccr     # Windows 下产物为 ccr.exe
```
把生成的二进制放进 PATH。多平台构建见 [RELEASING.md](RELEASING.md)。

## 用法

| 命令 | 说明 |
|---|---|
| `ccr` | 交互式选择一个配置并启动（方向键 + 输入过滤） |
| `ccr <名字\|别名\|前缀> [claude 参数]` | 按名/别名/模糊命中直启，多余参数透传给 `claude` |
| `ccr -` | 重跑上次用的配置 |
| `ccr .` | 跑默认配置（先用 `ccr default` 设过） |
| `ccr ls` | 列出全部配置（带来源标记，token 不显示） |
| `ccr show <名字> [--reveal]` | 查看配置（默认 token 打码，`--reveal` 显示完整） |
| `ccr edit <名字>` | 用 `$EDITOR` 编辑/新建自定义配置 |
| `ccr alias [<别名> <目标>]` | 无参列出别名；带参设置别名 |
| `ccr unalias <别名>` | 删除别名 |
| `ccr default [<名字>]` | 无参查看默认；带参设置默认 |
| `ccr completion <shell>` | 打印补全脚本（bash/zsh/powershell） |
| `ccr completion install [shell] [--uninstall]` | 一键装/卸补全到当前 shell 配置 |
| `ccr doctor [名字]` | 体检后端可达性（HTTP 探测 ANTHROPIC_BASE_URL，<500 算通；不带名=全部，有不通则退出码非 0） |
| `ccr chain <file> [--auto]` | 跑一条多后端流水线（yaml 描述）；`--auto` 不停顿一路跑完 |
| `ccr chain init [模板名]` | 从内置模板生成 `<模板名>.chain.yaml`（默认模板 plan-impl-review） |

名字在两个来源中冲突时，用限定名消歧：`ccr cc-switch:DeepSeek` 或 `ccr custom:DeepSeek`。

### 别名 / 默认 / 上次

让常用配置更顺手——这些元数据旁挂在 `~/.ccr/`，不动 cc-switch 库：

```
ccr alias prod cc-switch:DeepSeek   # 设别名，之后 `ccr prod` 直启
ccr default my-local                # 设默认，之后 `ccr .` 直启
ccr -                               # 重跑上次用的配置
ccr de                             # 模糊命中：唯一则直启，多个则弹选择器
```

别名/默认存 `~/.ccr/overlay.json`，上次用的存 `~/.ccr/state.json`。

`ccr <参数>` 的解析顺序：精确名/限定名 → 别名 → 模糊子串（唯一直启、多命中弹过滤选择器、零命中报错）。

### Shell 补全

**通过 Scoop / Homebrew 安装的，补全已自动注册，无需手动触发**——安装钩子会替你跑 `ccr completion install`，重开终端即生效（升级幂等，卸载后也不会报错）。

其他安装方式（`go install`、下载二进制等）手动跑一次即可。`ccr completion install` 与安装方式无关，往当前 shell 的配置文件幂等写入一段引导（重复跑不重复写）：

```
ccr completion install              # 自动探测 bash/zsh/powershell
ccr completion install zsh          # 也可显式指定
ccr completion install zsh --uninstall   # 干净卸载
ccr completion bash > /某处          # 或自取脚本手动放置
```

补全里的配置名是动态的（脚本运行时调 `ccr __complete_names` 取当前列表）。

## 多后端流水线（chain）

把多个 provider 串成一条流水线：A 段跑完交给 B、B 给 C，每段可挂不同后端（如强模型规划 → 便宜模型实现 → 另一家审查）。

```
ccr chain init                 # 生成 plan-impl-review.chain.yaml
# 改好里面三段的 profile 名（用 ccr ls 看可用名）
ccr chain plan-impl-review.chain.yaml          # 默认每段间停下放行
ccr chain plan-impl-review.chain.yaml --auto   # 一条道跑到黑，不停顿
```

**段间放行**：每段跑完停在 `⏸`，回车=放行下一段 / `s`=跳过 / `e`=改指令 / `q`=退出。停顿时可直接编辑工作目录里的产出文件再放行。

**chain.yaml 字段**：

- 顶层：`name`、`isolate`（true=在临时 git worktree 跑，可整体回滚）、`segments`。
- 每段：`profile`（ccr 配置查询名）、`prompt`（可含 `{{prev.output}}` 注入上段输出）、`allow_tools`（claude 工具白名单）、`deny_commands`（追加到内置命令黑名单，命中即拦）、`review`（true=该段写判定到 `.ccr-chain/verdict`）、`optional`（可选段，非 --auto 时在放行点决定是否跑）。

**安全**：worktree 隔离可回滚 + 每段工具白名单 + PreToolUse 钩子拦截命令黑名单 + 写操作圈在工作目录内。⚠️ Windows 真沙箱弱，主要靠 worktree 回滚 + 钩子黑名单兜底。

**先体检**：`ccr doctor` 跑链前确认各后端可达。

## 配置来源

合并自两处，列表里以 `[cc-switch]` / `[custom]` 区分：

1. **cc-switch 库**（默认 `~/.cc-switch/cc-switch.db`，只读）——自动读取其中 `app_type=claude` 的全部 provider。
2. **自定义目录** `~/.ccr/profiles/*.json`，每个文件一个配置，文件名即配置名，内容与 cc-switch 的 `settings_config` 同构：

   ```json
   {
     "model": "sonnet",
     "env": {
       "ANTHROPIC_BASE_URL": "https://api.example.com/anthropic",
       "ANTHROPIC_AUTH_TOKEN": "sk-..."
     }
   }
   ```

未安装 cc-switch 时，仅使用自定义目录。

## 路径覆盖

优先级：环境变量 > `~/.ccr/config.json` > 默认值。

- `CCR_DB`：cc-switch 库路径
- `CCR_PROFILES_DIR`：自定义 profiles 目录

`~/.ccr/config.json`：

```json
{ "db": "C:\\path\\cc-switch.db", "profilesDir": "D:\\my-profiles" }
```

## 开发

开发环境、`task` 命令、提交规范、git 钩子见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 构建与发布

纯 Go 实现（`modernc.org/sqlite` 免 cgo），单文件二进制，无运行时依赖。

发布走 [GoReleaser](https://goreleaser.com)：打一个 `v*` tag 推上去，GitHub Actions 会自动构建全平台、发 Release、并更新 Scoop / Homebrew。完整流程与一次性设置见 [RELEASING.md](RELEASING.md)。

本地多平台试构建：
```
goreleaser release --snapshot --clean   # 产物在 dist/，不发布
```
