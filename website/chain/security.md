# 安全

chain 让 AI 在你的文件系统上自动执行多段操作，安全不是可选项。CC RUN chain 采用**四层纵深防御**，层层递进。

## 四层防御

```
第一层  工作区隔离（git worktree / copydir 快照）
  ↓  哪怕出问题，原始文件不受影响、可回滚
第二层  工具白名单（per-segment allow-tools）
  ↓  每段只放行它真正需要的工具
第三层  PreToolUse 命令红线（内置默认 + 用户追加）
  ↓  危险命令被拦截在工具调用之前
第四层  写操作限定在工作目录内
  ↓  不允许向系统目录或其他项目写入
```

### 第一层：工作区隔离

已在 [隔离与成果交回](./isolation) 中详述。简单说：Git 仓库创建临时 worktree，非 Git 目录复制快照。原始文件不受影响，任何操作都可回滚。AI 跑在沙箱里，不是你的主工作区。

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

### 第三层：命令红线（deny_commands）

即使 AI 能执行 Bash，某些命令也绝不能让它碰。这是 PreToolUse 钩子实现的——在工具真正调用之前拦截。

**内置默认红线**（CC RUN 内置，始终生效）：

| 命令模式 | 阻挡原因 |
|----------|----------|
| `rm -rf /` 或 `rm -rf /*` | 递归删除根目录 |
| `rm -rf ~` 或 `rm -rf $HOME` | 删除用户主目录 |
| `git push --force` / `git push -f` | 强制推送 |
| `git reset --hard` | 硬重置（丢工作区变更） |
| `git clean -fdx` | 删除所有未跟踪文件 |
| `chmod 777`（递归） | 权限过度放开 |
| `curl ... \| sh` / `curl ... \| bash` | 管道执行远程脚本 |
| `eval` | 动态执行字符串 |
| `sudo` | 提权操作 |

**用户追加**：在 chain.yaml 的 segment 定义中添加 `deny_commands` 字段，叠加到内置默认之上：

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

一旦命中红线，引擎立即中止当前 segment，不会继续执行。

### 第四层：目录边界

所有写操作（Write、Bash 中的文件写入）限定在工作目录（worktree 或 copydir）内。AI 不能向系统目录（`/etc`、`/usr`、`C:\Windows`）或其他项目写入。

## Windows 上的现实

::: warning Windows 沙箱能力有限
Linux / macOS 可以用容器、seccomp、chroot 等技术做真正的进程级沙箱。Windows 上这些手段不成熟或不可用。因此，Windows 平台的安全**主要依赖第一层（worktree 回滚）加第三层（命令红线）**。

如果你在 Windows 上跑 chain，建议：
- 确保工作目录在 Git 仓库中（启用 worktree 隔离）
- 仔细配置 `deny_commands`
- 对高风险链先用 `--dry-run` 看一遍 prompt 再真跑
:::

## 最佳实践

::: tip 安全清单
- **always isolate**：不要设 `isolate: false`，除非你完全信任这条链且手动验证过
- **最小权限**：每个 segment 的 `allow_tools` 只放行它真正需要的，不给多余权限
- **红线只加不减**：内置 `deny_commands` 覆盖最危险的模式，你的追加只应该更严，不要试图"放松"
- **先 dry-run**：新链先 `--dry-run` 看一遍每段会发什么 prompt，确认没有意外的危险指令
- **审查第一**：无论多信任这条链，审查段（`review: true`）是必选项，不要删
:::

## 下一步

→ [命令速查](../reference/commands)
