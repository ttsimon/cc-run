# 安全

chain 让 AI 在你的文件系统上自动执行多段操作，安全不是可选项。CC Run chain 采用**纵深防御**，层层递进。

## 防御分层

```
第一层  工作区隔离（git worktree / copydir 快照）
  ↓  哪怕出问题，原始文件不受影响、可回滚（需显式 isolate: true）
第二层  工具白名单（per-segment allow_tools）
  ↓  每段只放行它真正需要的工具
第三层  PreToolUse 守卫的三道闸（__chain_guard）
  ↓  命令红线 + cd 上跳拦截 + 路径围栏，都在工具调用之前生效
第四层  干净运行环境
  ↓  每段空的 CLAUDE_CONFIG_DIR + 守卫配置放在工作目录之外，agent 够不着、改不了自己的红线
```

### 第一层：工作区隔离

已在 [隔离与成果交回](./isolation) 中详述。简单说：Git 仓库创建临时 worktree，非 Git 目录复制快照。原始文件不受影响，任何操作都可回滚。AI 跑在沙箱里，不是你的主工作区。

::: warning 隔离默认是关的
`isolate` 缺省为 `false`——省略它，链就直接在你的当前目录跑，这一层防御不生效。**务必在 chain.yaml 里显式写 `isolate: true`**（内置模板已写好）。
:::

### 第二层：段级工具白名单

在 chain.yaml 的 segment 定义中，通过 `allow_tools` 限制本段能用的工具：

```yaml [限制工具白名单]
segments:
  - name: plan
    profile: strong
    prompt: "规划并写计划到 docs/plans/"
    allow_tools:
      - Read
      - Write
      - Bash
      - Glob
      - Grep
```

没有列在 `allow_tools` 里的工具，本段无权调用。规划段不需要 `Task`、`WebFetch` 之类的能力就不要给。

### 第三层：PreToolUse 守卫的三道闸

每段都挂一个 PreToolUse 钩子（`ccr __chain_guard`），在**每次工具调用之前**运行。它有三道闸，任一命中就以退出码 `2` **拦下这次工具调用**（命令不执行、文件不读写），agent 拿到拒绝结果后只能换别的做法。

**闸一 · 命令红线（deny_commands）**

内置一组红线，**子串匹配、大小写不敏感**（大小写不敏感是为了挡 Windows PowerShell 的大写变体）。内置默认始终生效：

| 命中子串 | 阻挡原因 |
|----------|----------|
| `rm -rf` | 递归强制删除（不限路径，全拦） |
| `git push` | 任何推送（不只是 force） |
| `shutdown` | 关机 / 重启 |
| `mkfs` | 格式化文件系统 |
| `:(){ :\|:& };:` | fork bomb |
| `dd if=` | 裸盘读写 |
| `> /dev/sd` | 直写块设备 |

在 segment 里加 `deny_commands` 可**追加**自己的红线（只增不减，叠加到内置之上）：

```yaml [追加命令红线]
segments:
  - name: impl
    profile: cheap
    prompt: "实现计划"
    deny_commands:
      - "npm unpublish"
      - "docker rm -f"
      - "aws s3 rm"
```

**闸二 · cd 上跳拦截**

挡掉明显跳出工作目录的 `cd`：`cd ..`、`cd /`、`cd ~`、`cd $HOME`（行首或 `; & |` 之后）。`cd ./subdir` 这种不离开 workdir 的不拦。

**闸三 · 路径围栏**

Read / Write / Edit / Glob / Grep / NotebookRead / NotebookEdit 等工具的路径参数（`file_path` / `path` / `pattern` / `notebook_path`）必须落在工作目录内，否则拦截。相对路径基于 workdir 解析，`..` 会被展开后再判断，软链也会被解析——绕不过去。系统目录（`/etc`、`C:\Windows`）和其他项目都在围栏之外。

需要个别例外，用 segment 的 `allow_paths` 开逃生口：

```yaml [放行额外路径]
segments:
  - name: build
    profile: cheap
    prompt: "构建并把产物写到 /tmp/out"
    allow_paths: ["/tmp"]
```

### 第四层：干净运行环境

- 每段跑在一个**空的 `CLAUDE_CONFIG_DIR`** 里，不继承你的全局插件 / 技能 / SessionStart 钩子——避免它们注入大段前言、诱发段去调技能或起子代理（行为不确定且费 token）。
- 守卫的 `--settings`（含 PreToolUse 钩子配置）写在**工作目录之外**的临时处。agent 的写操作圈在 workdir 内，够不着这里，因而**看不见、也改不了自己头上的红线钩子**。
- 同时设 `GIT_CEILING_DIRECTORIES`，把 agent 自己跑的 `git` 锁在工作目录内，不会爬到父仓库。

## Windows 上的现实

::: warning Windows 沙箱能力有限
Linux / macOS 可以用容器、seccomp、chroot 等技术做真正的进程级沙箱。Windows 上这些手段不成熟或不可用。因此，Windows 平台的安全**主要依赖第一层（worktree 回滚）加第三层（命令红线）**。

如果你在 Windows 上跑 chain，建议：
- 确保工作目录在 Git 仓库中，并显式写 `isolate: true`（启用 worktree 隔离）
- 仔细配置 `deny_commands`
- 高风险链先**不要加 `--auto`**：逐段放行，每段跑完检查产出再继续
:::

## 最佳实践

::: tip 安全清单
- **显式开隔离**：`isolate` 默认是 `false`，记得写 `isolate: true`——别依赖默认值，否则链直接在你的工作目录里跑
- **最小权限**：每个 segment 的 `allow_tools` 只放行它真正需要的，不给多余权限
- **红线只加不减**：内置 `deny_commands` 覆盖最危险的模式，你的追加（含 `allow_paths`）只应该更严，不要试图"放松"
- **新链先逐段放行**：第一次跑别加 `--auto`，每段跑完看一眼产出和工具调用再继续，确认没有意外
- **审查第一**：无论多信任这条链，都放一个审查段（`review: true`）——它同时也是隔离区「自动合并 / 保留」的质量闸，没有它链会跑完直接合并
:::

## 下一步

→ [命令速查](../reference/commands)
