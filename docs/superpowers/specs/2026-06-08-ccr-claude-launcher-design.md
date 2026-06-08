# ccr — 跨平台 Claude 会话启动器（设计文档）

- 日期：2026-06-08
- 状态：已通过头脑风暴评审，待用户复核
- 命令名：`ccr`（cc-run）

## 1. 背景与问题

cc-switch 是一个切换 **全局** 当前 Claude provider 的 GUI 工具——同一时刻只有一个 provider 生效，配置写进全局 `~/.claude`。

但 Claude Code 本身是通过 **环境变量**（`ANTHROPIC_AUTH_TOKEN`、`ANTHROPIC_BASE_URL`、各种模型映射）来决定走哪个后端的。这意味着：只要在某个终端 tab 里设好这套环境变量再启动 `claude`，就能让那个会话独立使用某个 provider，**而不触碰全局配置**。

`ccr` 就是把这件事做成一条命令：**给当前终端注入选定 provider 的环境变量，然后拉起 `claude`**。由此可以**同时多开**多个终端，每个用不同的厂商/模型，互不干扰。

## 2. 目标

1. 一条命令在当前终端注入某个 provider 的 env 并启动 `claude`。
2. 默认从已安装的 **cc-switch** 读取全部 Claude provider（只读）。
3. 在 cc-switch 之外，支持一个**自定义 profiles 目录**作为第二来源（也用于未安装 cc-switch 的场景）。
4. 两个来源合并成一个列表，**带来源标记**。
5. 两种选择方式：不带参数弹交互式 fuzzy 清单；带参数按名字直启。
6. 跨平台：Windows (PowerShell)、macOS、Linux，行为一致。
7. 单文件二进制分发，无运行时依赖。

## 3. 非目标（v1 明确不做，YAGNI）

- 自动弹出新的终端窗口/标签页（依赖各终端私有 API，且 Warp on Windows 不支持——交给终端自己开 tab）。
- 工具内 `add` / `rm` 配置向导（自定义 profile 用 `ccr edit` 拉编辑器手写）。
- 写回 / 修改 cc-switch 的数据库或全局当前 provider（永远只读 cc-switch）。
- 非 Claude 的 app_type（codex / gemini / claude-desktop）。
- failover、用量统计、代理等 cc-switch 的高级特性。

## 4. 核心概念：按会话注入环境变量

`ccr <profile>` 的本质：

```
最终环境 = 当前 shell 环境 ∪ profile.env（profile 覆盖同名变量）
然后以子进程拉起 claude，继承 stdin/stdout/stderr，透传退出码。
```

二进制本身不修改父 shell，只是用修改过的环境拉起 `claude` 子进程。多开 = 用户在终端里开多个 tab，各跑一次 `ccr <profile>`。

## 5. 命令形态

| 命令 | 行为 |
|---|---|
| `ccr` | 弹交互式 fuzzy 清单（两来源合并、带来源标记）→ 选中即启动 |
| `ccr <名字> [claude 参数...]` | 按名字直启；`--` 之后或多余参数透传给 `claude` |
| `ccr ls` | 列出所有 profile（名字、来源、模型、Base URL 主机名），不启动 |
| `ccr show <名字>` | 查看某个 profile 的完整内容（**两来源都能查**，token 打码） |
| `ccr edit <名字>` | 用 `$EDITOR` 打开/新建 `~/.ccr/profiles/<名字>.json`（**仅自定义来源可写**；若名字命中 cc-switch provider 则报错说明不可改） |

`ccr --version` / `ccr -h` 常规。

## 6. 架构（三层，职责单一）

```
Sources(来源层)        Registry(合并层)         Launcher(启动层)
 ├ ccswitch  ─┐                                
 │            ├─►  合并 + 去重/标记来源  ─►  组装 env + 拉起 claude
 └ customdir ─┘
```

### 6.1 Profile 数据模型

```go
type Source string // "cc-switch" | "custom"

type Profile struct {
    Name      string            // cc-switch 的 provider name，或自定义文件名(去扩展名)
    Source    Source
    Model     string            // settings_config 顶层 "model"，可空
    Env       map[string]string // settings_config.env
    BaseURL   string            // 从 Env["ANTHROPIC_BASE_URL"] 提取，仅用于展示主机名
    IsCurrent bool              // 仅 cc-switch：是否为 is_current（清单里加个标记）
}
```

### 6.2 来源层接口

```go
type ProfileSource interface {
    Available() bool          // 该来源是否存在（如 cc-switch 库文件是否在）
    Load() ([]Profile, error) // 解析为 Profile 列表
}
```

- **ccswitch 源**：只读打开 `~/.cc-switch/cc-switch.db`，查
  `SELECT name, settings_config, is_current FROM providers WHERE app_type='claude'`，
  解析每行 `settings_config`（JSON：`{"model":..., "env":{...}}`）。`Available()` = 库文件存在。
- **customdir 源**：遍历 `~/.ccr/profiles/*.json`，每个文件解析为 `{model, env}`，名字 = 文件名去扩展名。`Available()` = 目录存在。

### 6.3 合并层

- 合并两个来源为一个列表。
- 名字冲突：**两条都保留**，各带来源标记；`ccr <名字>` 命中多个时，提示并要求用 `cc-switch:<名字>` / `custom:<名字>` 形式消歧（或在 TUI 里选）。
- 排序：自定义在前或按名字字典序（实现时定，清单里分组显示来源）。

## 7. 数据流

```
启动 ccr [名字] [claude参数...]
 → 构造 ccswitch 源 + customdir 源
 → 对 Available() 的源调用 Load()（某源不可用则跳过）
 → 合并为列表
 → 若给了名字: 精确匹配 → 命中唯一则用之；命中多个 → 消歧；没命中 → 列出相近名字并退出
   否则: 渲染 TUI fuzzy 清单 → 用户选一个
 → 组装环境: os.Environ() 叠加 profile.Env（profile 覆盖）
 → 若 profile.Model 非空且 Env 未显式指定模型 → 给 claude 追加 --model <Model>
 → 解析 PATH 中的 claude → 以子进程启动，继承三个标准流
 → 等待并透传退出码（含信号转发）
```

## 8. 配置与路径（均可被 flag / 环境变量覆盖）

| 项 | 默认值 | 覆盖方式 |
|---|---|---|
| cc-switch 库 | `~/.cc-switch/cc-switch.db`（三平台同） | `--db` / `CCR_DB` |
| 自定义 profiles 目录 | `~/.ccr/profiles/` | `--profiles-dir` / `CCR_PROFILES_DIR` |
| ccr 自身配置 | `~/.ccr/config.json`（可选） | `--config` / `CCR_CONFIG` |

> 注：用户在评审中点选过 `~/.ccx/profiles/`，但为与命令名 `ccr` 一致改为 `~/.ccr/profiles/`。如需改回 `~/.ccx` 一处即可。

`~/.ccr/config.json`（全部可选）用于覆盖上面两个路径，例如：

```json
{
  "db": "C:\\custom\\cc-switch.db",
  "profilesDir": "D:\\my-claude-profiles"
}
```

## 9. cc-switch 数据库访问（关键技术点）

cc-switch 是运行中的 GUI，可能正握着该 SQLite 文件。

- 采用 **纯 Go 的 `modernc.org/sqlite`**（免 cgo，便于交叉编译）。
- 以 **只读** 方式打开：DSN 形如 `file:<path>?mode=ro&immutable=1&_pragma=busy_timeout(2000)`。
- 只读 + immutable 不与 cc-switch 抢写锁；SQLite 天生支持并发读。
- 兜底：若仍因锁失败，把 `.db`（及可能的 `-wal`/`-shm`）拷到临时文件再只读读取。
- 永不写入该库。

自定义来源不依赖 SQLite，只读 JSON 文件。

## 10. 自定义 profiles 目录

- 每个文件一个 profile，扩展名 `.json`，文件名（去扩展名）= profile 名 = 子命令名。
- 文件内容与 cc-switch 的 `settings_config` 同构：

```json
{
  "model": "sonnet",
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "sk-...",
    "ANTHROPIC_BASE_URL": "https://api.example.com/anthropic",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "..."
  }
}
```

- `ccr edit <名字>` 在文件不存在时用一个**带占位字段的模板**创建后再拉起 `$EDITOR`。

## 11. 启动层（跨平台拉起 claude）

- 用 `os/exec` 启动 `claude`，`Stdin/Stdout/Stderr` 直接继承当前进程的三个流（保证 TUI 交互体验）。
- 透传退出码：等待子进程结束，用其 ExitCode 退出。
- 信号：转发 Ctrl-C 等给子进程（Windows 与 Unix 分别处理）。
- 找不到 `claude`（不在 PATH）→ 明确报错并提示安装/配置方式。
- 说明：Windows 上没有真正的 `exec` 替换语义，统一采用「子进程 + 等待 + 透传退出码」，三平台行为一致。

## 12. 错误处理

| 情况 | 行为 |
|---|---|
| 未装 cc-switch（库不存在） | 静默跳过该来源，仅用自定义 |
| 两来源都为空 | 友好提示：如何在 `~/.ccr/profiles/` 加自定义 profile（给示例） |
| 库被锁 | 只读/immutable 打开；再失败则拷临时文件读 |
| 某个自定义 JSON 解析失败 | 警告并跳过该文件，其余照常加载 |
| `ccr <名字>` 没匹配上 | 列出相近名字后退出（非零码） |
| `ccr <名字>` 命中多个来源 | 提示用 `来源:名字` 消歧 |
| `claude` 不在 PATH | 明确报错 |

## 13. 安全

- 任何列表 / `show` / 日志输出**绝不打印完整 token**（保留前缀如 `sk-...`）。`show` 默认打码，可加 `--reveal` 显式查看。
- `ccr edit` 新建/写入的文件权限设为 `0600`（Windows 上设置等价 ACL，best-effort）。
- 不把 token 写入任何日志文件。

## 14. 测试策略（TDD）

每层独立可测：

- **解析**：用真实样本（火山 Coding Plan、DeepSeek）验证 `settings_config` → Profile；自定义 JSON 解析；坏 JSON 跳过。
- **来源探测**：临时 sqlite fixture（写入若干 providers 行）+ 临时 profiles 目录，验证 `Available()` 与 `Load()`。
- **合并层**：去重 / 重名消歧 / 来源标记 / 排序。
- **env 组装**：覆盖顺序（profile 覆盖系统、env 显式模型时不再追加 `--model`）。
- **启动层**：注入一个假的 `claude`（回显环境变量的脚本/stub 二进制），断言 env 正确传入、参数透传、退出码透传。跨平台 stub。
- **安全**：token 打码逻辑、文件权限。

## 15. 构建与分发

- Go + `CGO_ENABLED=0`，`GOOS`/`GOARCH` 矩阵交叉编译出 Windows/macOS/Linux 单文件二进制。
- UI 库：charmbracelet（`huh` 做 fuzzy 选择清单，必要时 `bubbletea`+`lipgloss`）。
- SQLite：`modernc.org/sqlite`（纯 Go）。
- CLI：标准库 flag 或 `cobra`（实现阶段定；命令面不复杂，倾向轻量）。

## 16. 未决 / 未来

- `ccr` 别名（如 `ccx`）是否需要——v1 仅 `ccr`。
- 名字冲突的默认优先级策略（当前为保留双方 + 消歧）。
- 未来若需：`add`/`rm` 向导、其他 app_type、写回 cc-switch。
