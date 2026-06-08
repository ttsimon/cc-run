# ccr

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

**5. winget（Windows，上架后）**
```
winget install ttsimon.ccr
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
| `ccr <名字> [claude 参数]` | 按名字直启，多余参数透传给 `claude` |
| `ccr ls` | 列出全部配置（带来源标记，token 不显示） |
| `ccr show <名字> [--reveal]` | 查看配置（默认 token 打码，`--reveal` 显示完整） |
| `ccr edit <名字>` | 用 `$EDITOR` 编辑/新建自定义配置 |

名字在两个来源中冲突时，用限定名消歧：`ccr cc-switch:DeepSeek` 或 `ccr custom:DeepSeek`。

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

## 构建与发布

纯 Go 实现（`modernc.org/sqlite` 免 cgo），单文件二进制，无运行时依赖。

发布走 [GoReleaser](https://goreleaser.com)：打一个 `v*` tag 推上去，GitHub Actions 会自动构建全平台、发 Release、并更新 Scoop / Homebrew / winget。完整流程与一次性设置见 [RELEASING.md](RELEASING.md)。

本地多平台试构建：
```
goreleaser release --snapshot --clean   # 产物在 dist/，不发布
```
