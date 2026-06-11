# ccr chain 隔离与成果交回 设计（track A）

日期：2026-06-11 ｜ 分支：feat/chain ｜ 范围：v0.3 / chain 发版前

## 动机（真实事故）

一次 `ccr chain plan-impl-review.chain.yaml`（`isolate: true`）生成了一个 HTML 落地页，跑完后**成果消失**。复盘：

- isolate 在临时 git worktree 里跑；
- implement 段（有 Bash）**碰巧自己 `git commit`** 了，成果进了临时分支；
- 但段配了 `deny_commands: ["git push"]`，成果只存在于**本地临时分支**；
- 链跑完，cleanup **无条件 `git worktree remove --force` + `git branch -D`**，把成果删了（只因提交成了悬空对象，靠 `git fsck` 才捞回）。

两个失败模式：agent 提交了 → ccr 删分支杀掉；agent 没提交 → `remove --force` 连未提交文件一起删（**真不可恢复**）。

**根因不是 deny push，而是 ccr 的 cleanup 无条件销毁成果**，且整个「落地/交回」依赖 agent 自觉。本设计重做隔离层，使成果**永不被静默销毁**，并在成功时自动合回用户仓库。

## 目标 / 非目标

**目标**
- 成功跑完 → ccr **本地** merge 成果到用户当前分支（带提交历史）。不 push。
- 失败 / 审查 needs-work / 中途退出 → 成果**保留**并打印取回路径，绝不销毁。
- 成果落地**不依赖 agent 自觉**：ccr 负责每段把改动落成提交。
- 支持**非 git 目录**（copydir 隔离）。
- 隔离机制**可插拔**，按上下文自动选。

**非目标（YAGNI）**
- 容器/VM 级安全隔离（留接口缝，不实现）。
- 显式「丢弃模式」旋钮（分支只在成果已安全进用户分支时才删）。
- 整合冲突的自动解决（撞了就回退到「保留 + 打印路径」）。
- copydir 的 `exclude` 配置、删除文件镜像（暂不处理删除）。
- 执行可观测性（track B，独立 spec）。

## 架构：可插拔 Isolator

orchestrator 不再直接调 worktree，改为依赖一个隔离接口：

```go
type Isolator interface {
    Setup() (workdir string, err error)   // 准备隔离工作区
    SealSegment(name string) error         // 一段跑完后：把该段成果落成持久形态
    Integrate() (summary string, err error) // 成功：把成果交回用户仓库
    Abandon() (location string, err error)  // 非成功：保留成果，返回取回位置
}
```

- **git 目录** → `worktreeIsolator`
- **非 git 目录** → `copydirIsolator`
- 选择在 orchestrator 进入隔离前做：`git rev-parse --is-inside-work-tree` 判定。
- 留缝：将来 `containerIsolator` 实现同一接口即可，不动编排。

不隔离（`isolate: false`）时不创建 Isolator，就地在真目录跑——行为不变。

## worktreeIsolator（git 目录）

- **Setup**：`git worktree add -b ccr-chain/<name>-<ts> <tmpdir>`，从当前 HEAD 长出临时分支，返回 tmpdir。
- **SealSegment**：段跑完后，若 worktree 有未提交改动（`git status --porcelain` 非空），ccr 执行 `git add -A && git commit -m "[ccr chain] <segment>"`。
  - agent 自己提了细粒度提交 → 无残留 → 不补，保留 agent 历史。
  - agent 没提 / 没 Bash（如 plan 段无 Bash 跑不了 commit）→ ccr 兜底全提。
  - **排除 `.ccr-chain/`**（settings/verdict/findings 运行产物）：在 worktree 根写一个 `.ccr-chain/.gitignore`（或提交时 pathspec 排除），不让运行产物合回用户仓库。
- **Integrate**：`git merge --ff-only ccr-chain/<...>` 合入用户当前分支；成功后 `git branch -D` 临时分支（已合入，删它不丢东西）+ `git worktree remove`。
  - 非 fast-forward（用户分支并行动过）→ 尝试普通 merge；冲突或工作树脏 → 转 **Abandon**。
- **Abandon**：**不删分支**。`git worktree remove`（仅删临时目录，提交已在分支上）。返回 `分支 ccr-chain/<...>`，打印 `成果在该分支，git merge 取回 / 不要可 git branch -D`。worktree 目录删除失败（Windows 占用）不影响成果。

## copydirIsolator（非 git 目录）

- **Setup**：把工作目录递归拷到 `os.TempDir()` 下的临时目录（保留原始快照即原目录本身），返回临时目录。
- **SealSegment**：无操作——文件就在临时目录里持久存在，天然不丢。
- **Integrate（成功）**：对比临时目录与原目录，把**新增/修改**的文件拷回原目录。
  - 判定「变了」靠内容比对（大小+内容）。
  - **不处理删除**：agent 在副本里删了文件，原目录对应文件**不动**（宁可多留，绝不替 agent 删用户文件）。
- **Abandon（非成功）**：不拷回、不删副本，返回临时目录路径，打印 `成果在 <临时目录>，自己查看/取用`。
- 取舍：大目录（node_modules 等）全拷会慢——先接受；非 git 无 .gitignore 可依赖，`exclude` 配置留待后续。单用户场景不做并发冲突检测，拷回即覆盖。

## 结束三态（两种 Isolator 统一语义）

「成功」= **所有段退 0 AND 审查段 verdict 为 pass**（无审查段则只看退出码）。

| 结束状态 | 处置 |
|---|---|
| 跑完 + pass（或无审查段） | **Integrate**：合回/拷回用户仓库 |
| 跑完 + needs-work | **Abandon**：当软失败，保留成果 + 打印路径，不自动塞进用户分支 |
| 某段报错 / 用户 `q` 退出 | **Abandon**：保留成果 + 打印路径 |

铁律：**只有 Integrate 成功后才删临时分支**；其余任何路径都保留成果。

## 落点

- **`internal/chain/isolate.go`**（新，吸收旧 `worktree.go`）：`Isolator` 接口 + `worktreeIsolator` + `copydirIsolator` + 上下文选择函数 `newIsolator(workdir, name)`。
- **`internal/chain/orchestrate.go`**：
  - 进入时按 `c.Isolate` + git 判定建 Isolator；
  - 每段跑完调 `SealSegment`；
  - 链结束按三态调 `Integrate` / `Abandon`，打印结果路径；
  - 删掉旧的「无条件 cleanup」。
- **`internal/chain/worktree.go`**：并入 isolate.go 后删除（或保留 `gitIn`/`sanitize` 工具函数）。
- **模板 `plan-impl-review.yaml`**：保留 `isolate: true`（现在已安全）。
- **文档**：`docs/superpowers/specs/.../ccr-chain-design.md` 与用户文档同步隔离新语义。

## 测试

- `worktreeIsolator`：Setup 建出 worktree 分支；SealSegment 在有/无残留时分别补提交/不补；`.ccr-chain` 不进提交；Integrate 把提交 ff 合入当前分支并删临时分支；ff 不成立或工作树脏时转 Abandon 且**分支仍在**。
- `copydirIsolator`：Setup 拷出；Integrate 把新增/修改拷回、删除不镜像；Abandon 保留副本返回路径。
- `orchestrate`：三态分支各走对 Integrate/Abandon；needs-work 不 Integrate；某段报错 → Abandon 且成果可寻；非 git 目录自动走 copydir。
- 事故回归：模拟「agent 没提交就结束」→ 成果仍在（worktree 经 SealSegment 兜底提交 / copydir 文件还在），**绝不出现静默销毁**。

## 风险

- worktree 隔离**只覆盖 git 跟踪文件**：未跟踪/ignore 的本地状态在新 worktree 里不存在、也不带回。produce 型任务（生成新文件）无碍；依赖未跟踪本地状态的任务需用 copydir 或不隔离。文档需讲明。
- `git merge --ff-only` 的前提是临时分支为当前 HEAD 后代——Setup 即从当前 HEAD 长出，单用户成立；并行改动走普通 merge / Abandon 兜底。
- copydir 拷贝成本对大目录偏高，后续以 `exclude` 缓解。
