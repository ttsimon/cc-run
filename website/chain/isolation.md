# 隔离与成果交回

默认情况下（`isolate: true`），ccr chain 不会在你的当前工作目录里直接跑——它会创建一个**临时工作区**，所有 segment 在里面操作，等一切完成后才决定是否把成果交回给你。

## 为什么要隔离

这是从一次事故中学到的教训。

旧版实现在"清理"阶段无条件删除了临时产物。一条链跑了三段，审查段判定 `needs-work`，但清理逻辑不管判定——直接把所有中间文件清掉了。用户想对照 findings 手动修复时，文件已经没了。

结论：**绝不静默销毁成果**。隔离的目的是安全——隔离区可回滚、可丢弃、但绝不偷偷删除。

## 两种隔离模式

| 场景 | 隔离方式 | 原理 |
|------|----------|------|
| Git 仓库 | **临时 worktree** | 基于当前分支创建临时分支，在其中工作，每段结束自动 commit |
| 非 Git 目录 | **copydir 快照** | 复制整个工作目录到临时路径，在副本中操作 |

Git worktree 是首选方案：创建快、不重复拷贝文件、可以随时切回去看原始状态、变更可追溯（每段一个 commit）。

## 三种终态

chain 跑完后，临时工作区面临三种可能的结局：

| 终态 | 触发条件 | 动作 |
|------|----------|------|
| 完成且通过 | 审查段 verdict = `pass` | **合并回当前分支** → 删除临时分支 |
| needs-work | 审查段 verdict = `needs-work` | **保留成果** → 打印取回路径 |
| 错误 / 退出 | 运行中异常或用户主动退出 | **保留成果** → 打印取回路径 |

::: danger 铁律
只有"整合成功"这一条路径会删除临时分支。**所有其他路径——needs-work、错误、退出——一律保留成果，打印找回路径**。成果绝不静默消失。
:::

::: warning worktree 的局限
Git worktree 只覆盖 Git 已跟踪的文件。以下内容不在隔离范围内：

- `.gitignore` 中的文件（如 `node_modules`、构建产物）
- 未跟踪的新文件（`git status --untracked`）

如果你的链需要严格的完整文件隔离，考虑在非 Git 场景下使用 copydir 模式，或手动确保关键文件已被 Git 跟踪。
:::

## 交回流程（Git worktree 场景）

```
chain 跑完 → verdict = pass
  → 用户点"继续"
    → 引擎将临时分支 merge 回当前分支
      → 删除临时 worktree + 临时分支
        → 输出 merge summary
```

如果 verdict = needs-work：

```
chain 跑完 → verdict = needs-work
  → 保留 worktree 在临时路径
    → 打印：
      "成果保留在 /tmp/ccr-chain-xxxxx/
       分支: ccr-chain/plan-impl-review
       切回: git worktree add /tmp/ccr-chain-xxxxx/ ccr-chain/plan-impl-review"
```

你可以手动检查、继续改，满意了再自己 merge。

## 下一步

→ [安全](./security)
