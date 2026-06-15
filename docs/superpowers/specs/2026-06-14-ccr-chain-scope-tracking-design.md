# ccr chain 任务范围追踪（相关文件集传递）设计

日期：2026-06-14 ｜ 分支：feat/chain ｜ 范围：v0.3 / chain

## 动机

chain 的后续段（尤其 review / fix）需要知道"本次需求到底动了哪些文件"。没有这个信息时，review 段会把整个工作目录甚至父仓库当审查对象——历史上 review 段把 ccr 自己当审查对象、就是这个根因。

目前靠两条临时手段顶着：
1. 模板里手写 prompt "只看当前工作目录，别读父仓库"——靠 agent 自觉，不可靠。
2. 未提交的**物理硬围栏**（`CCR_CHAIN_WORKDIR` + `PathEscapes` + `BashEscapesWorkdir` + `GIT_CEILING_DIRECTORIES` + `allow_paths`）——挡住了越界，但只解决"不能跑出 workdir"，没解决"workdir 内那么多文件，该聚焦哪些"。

本设计加一层**任务边界**：orchestrator 在段间传递"本次链已改动的文件集"，注入后续段 prompt，把审查/修复范围收敛到真正相关的文件上。

## 三层边界（本设计在第 2 层上加第 3 层）

| 层 | 机制 | 状态 |
|---|---|---|
| 1. 隔离层 | worktree / copydir / 就地，成果不被销毁 | 已有 |
| 2. 物理硬围栏 | guard 拦 workdir 外路径 + cd 上跳；`GIT_CEILING` 防 git 爬父仓库 | 已有（未提交 diff，**保留**） |
| **3. 任务边界（本设计）** | 段间传递"相关文件集"，注入后续段 prompt 作软提示 | 新增 |

**与硬围栏的关系**：物理硬围栏是安全地板（绝对边界，保留），本设计是叠加的范围提示。**guard 不因本设计改变**——按已拍板的决策，guard 只做 workdir 硬围栏，workdir 内相邻文件读取放行（信任 prompt 约束），相关文件集**不进 guard**、纯 prompt 层软提示。`allow_paths` 作为显式 opt-in 逃生口保留。

## 已拍板的两个决策

1. **guard 用法 = workdir 硬围栏 + 文件集软提示**：guard 物理挡 workdir 外；文件集只注入 prompt 提示"聚焦这些、勿读无关文件"；workdir 内相邻读取放行。
2. **文件集累积范围 = 链级总集**：每段改动并入一个只增的总集，后续段看到的是 1..i-1 段的全部改动并集。

   ```
   plan   → (无改动)        seg2 prompt 看到: {}
   impl   → +foo.go +bar.go  seg3 prompt 看到: {foo.go, bar.go}
   fix    → +baz.go          末段看到: {foo.go, bar.go, baz.go}
   ```

## 核心抽象：ChangeTracker

按工作目录是否在 git 工作树内，选 git diff 还是文件系统快照——与 Isolator 的 git/非 git 判定同源（`isInsideWorkTree`），但**独立于 Isolator**：`isolate: false` 就地执行时没有 Isolator，仍需追踪。

```go
type ChangeTracker interface {
    // Baseline 在链开始时记录基准（git: HEAD + 已脏文件集；fs: 文件 size/mtime 快照）。
    Baseline() error
    // ChangedFiles 返回相对基准、被本链改动的文件（相对 workdir 的路径，已排序）。
    ChangedFiles() ([]string, error)
}

func newChangeTracker(workdir string) ChangeTracker // 内部用 isInsideWorkTree 选实现
```

### gitTracker（workdir 在 git 工作树内）

涵盖两种执行模式，逻辑统一：
- **worktree isolate**：每段 SealSegment 会提交；`git diff --name-only <baseSHA>` 比较 baseSHA 与当前工作树，**含已提交的封存改动**。
- **就地 git（isolate=false）**：不提交，改动留在工作树；同一条 `git diff --name-only <baseSHA>` 直接捕获。

```
Baseline:
  baseSHA   = git rev-parse HEAD
  baseDirty = set(git status --porcelain 的文件名)   // 链开始前已脏的文件
ChangedFiles:
  tracked   = git diff --name-only <baseSHA>          // 工作树(含已提交) vs baseSHA，不含 untracked
  untracked = git ls-files --others --exclude-standard
  return sort((tracked ∪ untracked) - baseDirty)      // 扣掉链开始前就脏的，只留本链动的
```

所有 git 调用走现有 `gitIn`——`GIT_CEILING_DIRECTORIES` 已防止从子目录爬到父仓库。

### fsTracker（非 git 目录 / copydir）

```
Baseline:
  snapshot = walk(workdir) -> map[相对路径](size, mtimeUnixNano)   // 跳过 .git / .ccr-chain
ChangedFiles:
  再 walk 一遍；某文件 新增 或 (size/mtime 与 snapshot 不同) 即视为改动
  return sort(改动文件)
```

用 size+mtime 而非内容 hash：追踪只需文件名，轻量即可，与隐患程度匹配（软提示，不追求精确）。

## Orchestrator 接入

```
进入隔离、workdir 定稿后：
  tracker = newChangeTracker(workdir); tracker.Baseline()    // best-effort，失败则后续不注入
  relevant = {}                                              // 链级只增总集

每段循环顶部（此时上一段已 SealSegment）：
  files, _ = tracker.ChangedFiles(); relevant ∪= files       // 累加并集（防"建了又删"）
  prompt = Render(...) [+ ReviewInstruction()]
  if len(relevant) > 0:
      prompt += RelevantFilesNote(sorted(relevant))          // 软提示，仅非空时
  ...照常跑段...
```

- 首段 relevant 为空，不注入。
- tracker 任何错误都不打断链（与 `segmentDiffStat` 同为 best-effort）。
- `RelevantFilesNote` 生成简洁提示，例如：

  ```
  [本次链已改动以下文件，请聚焦这些文件，不要读取无关或外部路径]
  - internal/chain/foo.go
  - internal/chain/bar.go
  ```

## 落点

- **新增 `internal/chain/changetracker.go`**：`ChangeTracker` 接口 + `gitTracker` + `fsTracker` + `newChangeTracker` + `RelevantFilesNote`。
- **新增 `internal/chain/changetracker_test.go`**：见测试。
- **改 `internal/chain/orchestrate.go`**：Baseline 初始化 + 循环顶部累加 + prompt 注入。
- **改模板 `internal/chain/templates/plan-impl-review.yaml`**：review 段手写的"只看当前工作目录"换成依赖注入的文件清单（保留一句简短引导即可）。
- **物理硬围栏 diff（未提交）**：保留不动——它是本设计的第 2 层地板。

## 测试

- `gitTracker`：临时 repo，Baseline 后新建/改文件 → ChangedFiles 含之；链开始前已脏的文件**被扣除**；untracked 文件**计入**；模拟 SealSegment 提交后改动**仍计入**（验证 `diff baseSHA` 覆盖已提交）。
- `fsTracker`：临时目录，Baseline 后新增/修改文件 → 返回之；`.git`/`.ccr-chain` 跳过。
- `newChangeTracker`：git 工作树返回 gitTracker，否则 fsTracker。
- `RelevantFilesNote`：空集返回空串；非空生成含全部文件名的提示。
- `orchestrate`：3 段链（注入的 fake runSegment 在 workdir 写文件），断言第 3 段 prompt 含前两段写的文件、首段 prompt 无清单；tracker 出错时链照常跑完。
- 回归（记忆里的事故）：复现"父级是 git、`temp/` 非 git"布局，review 段拿到的相关文件集**不含父仓库文件**。

## 非目标（YAGNI）

- guard 按文件集做硬限制——明确不做，workdir 内相邻读取放行。
- `{{changed.files}}` 显式占位符——MVP 自动追加，精确放置留后续。
- 重命名检测、删除精确镜像、内容 hash 级变更判定。
- 段级"本段增量"展示——worktree 已有 `segmentDiffStat`，不动。
