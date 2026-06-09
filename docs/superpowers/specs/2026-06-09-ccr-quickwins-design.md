# ccr 快点小功能批 设计（v0.2）

> 状态：设计已定稿（2026-06-09 brainstorm 通过）。**这是下一个动手的版本**，先于 chain（chain = v0.3，见 `2026-06-09-ccr-chain-design.md`）。
> 价值：便宜、独立、立刻能用；且铺的 overlay 元数据层是 chain 之后也要复用的基建。

## 范围

一批小功能，共用一层 overlay 元数据：

- 别名（alias）
- 默认 profile（default）
- 上次用的 profile（last，自动记录）
- CLI 名字模糊命中（fuzzy resolution）
- shell 补全（completion）

C 档搁置（不在本版）：终端标签页改名(OSC)、`ccr which`/`CCR_PROFILE`、`ccr env`、codex/gemini 后端。

## 为什么要 overlay 层

cc-switch 的 SQLite 库是**只读**来源，别名 / 默认 / 上次这些用户旁挂的元数据没法写回去。所以在 `~/.ccr/` 下另开两份文件：

- `~/.ccr/overlay.json`：用户显式设的元数据——别名表、默认 profile。
- `~/.ccr/state.json`：运行时自动记录的状态——上次成功拉起的 profile。

两份分开：overlay 是用户意图（可纳入备份/同步），state 是机器本地运行痕迹（可随时丢）。

## 地基决策：无参 `ccr` 不变

`ccr`（不带任何参数）**维持现状 = 弹 TUI 模糊选择器**。默认/上次只做成显式入口，不抢无参行为。好处：无优先级谜题、不改老行为。

## `ccr <参数>` 解析顺序

按精确→模糊逐级匹配：

1. **特殊记号**：
   - `ccr -` → 上次用的（读 `state.json`，没有则报错提示）。
   - `ccr .` → 默认（读 `overlay.json`，没设则报错提示）。
   - 用记号而非占用普通名字，避免和 profile/别名重名。
2. **精确匹配 profile 名**（含 `来源:名字` 消歧形式，沿用现有 registry 解析）。
3. **精确匹配别名**（overlay.json 的别名表）。
4. **模糊 / 前缀命中**：
   - 唯一命中 → 直接拉起。
   - 多个命中 → 弹**已过滤的 TUI**（把候选喂给现有 huh 选择器），不报错。
   - 零命中 → 报错，带「你是不是想找 X」之类提示。

> 解析逻辑是纯函数，单元测试覆盖（沿用项目「纯逻辑全单测」约定）。

## 配套命令

写 overlay 的命令：

- `ccr alias <别名> <profile>` — 设别名。
- `ccr alias` — 列出所有别名。
- `ccr unalias <别名>` — 删别名。
- `ccr default <profile>` — 设默认。

「上次」无需命令：每次成功拉起后自动写 `state.json`。

## shell 补全

项目用自定义 cli 分发（非 cobra），补全脚本自己生成（静态脚本，不依赖框架自带补全）。

**覆盖：各平台默认 shell 全覆盖** —— PowerShell（Windows）+ bash（Linux）+ **zsh（macOS，Catalina 起默认）**。推迟 fish（少数派、无平台以它为默认）。

补全内容：子命令（alias/default/completion/...）+ 可选 profile 名 + 别名。

### 安装：`ccr completion install` 为核心，包管理器装机自动调用它

> 📝 **演进（计划→实现阶段）**：Homebrew 走 **Cask**、Scoop 也没有 Formula 那种「补全目录」机制——所以不能用「补全目录」自动装。但二者都支持**安装后钩子**，于是改用钩子去调我们自己的 `ccr completion install`，「装包即带补全」仍然实现了，只是机制不同。

1. **`ccr completion install [shell]`（核心，一条命令）**：自动探测当前 shell（不带 shell 参数时）→ 往该 shell 的配置文件**幂等追加一段带标记的引导块**。重复跑不重复写；`--uninstall` 删掉标记之间的块、干净撤回。
   - rc 落点：bash→`~/.bashrc`、zsh→`~/.zshrc`、PowerShell→`~/Documents/PowerShell/Microsoft.PowerShell_profile.ps1`。
   - 引导行带**存在性兜底**：`if (Get-Command ccr …)` / `command -v ccr && …`——ccr 卸载/换版本/不在 PATH 时静默跳过，不污染 shell 启动。
2. **包管理器装机即带（基于钩子，非补全目录）**：
   - Scoop：manifest 的 `post_install` 跑 `ccr completion install powershell`（GoReleaser `scoops.post_install` 生成）。`scoop install` 完补全即生效，升级幂等无害。
   - Homebrew Cask：`hooks.post.install` 里在去隔离属性后跑 `ccr completion install`（探测 $SHELL 写 ~/.zshrc 或 ~/.bashrc）。
   - 卸载后即使 rc 残留引导行，靠①的存在性兜底静默跳过；不依赖包管理器的卸载钩子。
3. **`ccr completion <shell>`（逃生口）**：纯输出脚本到 stdout，给想自己控制放置位置的人（`ccr completion bash > /某处`）。

> 说明一：动态的 profile 名/别名由脚本运行时调用隐藏命令 `ccr __complete_names` 取得，故脚本本身是静态的。
> 说明二：包管理器钩子的真实安装行为需发一个 tag 才能端到端验证；`goreleaser check` + `--snapshot` 已验证生成的 manifest 含 `post_install`。

## 架构落点（对应现有 internal/ 布局）

- `internal/source/` 或新 `internal/overlay/`：读写 `~/.ccr/overlay.json`、`state.json`。
- `internal/registry/`：扩展按名解析，纳入别名表与特殊记号、模糊命中。
- `internal/cli/`：新增 `alias`/`unalias`/`default`/`completion` 子命令；无参与 `<参数>` 分发接上新解析。
- `internal/config/`：overlay/state 文件路径解析（沿用 env > `~/.ccr/config.json` > 默认 的优先级思路）。
- `internal/tui/`：复用现有选择器，新增「喂候选子集」入口供模糊多命中调用。

## 怎么算成功

- 设了别名后 `ccr <别名>` 直接拉起对应 provider。
- `ccr -` 重跑上次、`ccr .` 跑默认。
- `ccr <前缀>` 唯一命中直接跑、多命中弹过滤 TUI。
- `ccr completion powershell` / `bash` / `zsh` 输出可用脚本，补出子命令与 profile 名。
- `ccr completion install` 在当前 shell 下一键装好补全（再跑幂等、`--uninstall` 能撤），与安装方式无关。
- 引导行带存在性兜底：ccr 不在 PATH 时静默跳过，不报错。
- Scoop/Homebrew 安装后经钩子自动注册补全（manifest 含 `post_install`，已用 goreleaser snapshot 验证）。
- 纯解析逻辑有单测；overlay/state 读写有单测。

## 明确不做（YAGNI）

- 无参 `ccr` 行为不变（不做默认/上次降级链）。
- 不做 fish 补全、不做 C 档功能。
