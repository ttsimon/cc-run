# 放行与审查

chain 默认不是全自动的——每个 segment 跑完后会停下来，让你检查产出再决定下一步。这叫"人在回路中"（human in the loop）。

## 默认行为：逐段暂停

每个 segment 执行完毕后，引擎进入 `⏸`（暂停）状态，给你四个选项：

| 操作 | 效果 |
|------|------|
| **继续** (continue) | 放行，进入下一段 |
| **编辑** (edit) | 先打开编辑器让你改当前段产出文件（如计划、代码），改完继续 |
| **跳过** (skip) | 跳过下一段——如果下一段是可选段则直接越过，否则退出 |
| **退出** (exit) | 终止链，保留所有已产出的文件 |

<div class="term">
<div class="term-bar">
<span class="term-dot red"></span><span class="term-dot yellow"></span><span class="term-dot green"></span>
</div>
<div class="term-body">
$ ccr chain plan-impl-review.yaml

[plan] 执行完成 → 产出 docs/plans/todolist.md

⏸ plan → impl
  [c] 继续  [e] 编辑计划  [s] 跳过 impl  [x] 退出
  >
</div>
</div>

暂停的意义在于：你不是把一切交给 AI 就两手一摊了——你在关键节点检查、修正方向、确保产出符合预期。

## --auto：跳过所有暂停

如果你信任这条链、不想逐段确认，加 `--auto`：

```bash [全自动运行]
ccr chain chain.yaml --auto
```

`--auto` 会跳过所有普通段的暂停，链端到端跑完。**但审查段即使 `--auto` 仍会暂停**，因为审查结果（pass / needs-work）需要人来做最终判断。

## 审查段的判定

审查段（`review: true`）结束时，Claude 会输出两样东西：

- **findings**：发现的问题列表（具体、可操作）
- **verdict**：总体判定——`pass`（通过）或 `needs-work`（需修改）

然后引擎进入 `⏸`。

```
[review] 执行完成
  verdict: needs-work
  findings: 3 项

⏸ review 完成
  verdict: needs-work
  [c] 继续  [e] 编辑 findings  [x] 退出
```

### 用户来决定，不由引擎自动分支

这是关键设计决策：

- **verdict = pass** → 你点"继续"，链结束，考虑整合成果
- **verdict = needs-work** → 你来决定：如果链里定义了 fix 段，继续就会跑 fix；如果没有，你也可以直接退出、手动处理

引擎**不会**根据 verdict 自动重跑、自动循环、自动分支。所有决策权在你手里。

::: tip 可选 fix 段
在 chain.yaml 里加一个 `optional: true` 的 fix 段，它只在审查段给出 needs-work、且你点"继续"时才会跑。如果审查通过（pass），fix 段自动跳过。这和"跳过下一段"的机制是一致的——可选段在决定跳过时不会报错。
:::

## 下一步

→ [隔离与成果交回](./isolation)
