# 放行与审查

chain 默认不是全自动的——每个 segment 跑完后会停下来，让你检查产出再决定下一步。这叫"人在回路中"（human in the loop）。

## 默认行为：逐段暂停

每跑完一段、且后面还有段时，引擎进入 `⏸`（暂停）状态，先回显上一段的产出，再给你四个选项：

| 按键 | 操作 | 效果 |
|------|------|------|
| **回车** | 放行 | 进入下一段（输入空、`y` 或任何无法识别的内容都按放行处理） |
| `s` | 跳过 | 跳过下一段，直接进入再下一段（任何段都能跳，不限于可选段） |
| `e` | 改指令 | 提示你输入**一行**新指令，用它替换下一段的 prompt，然后放行 |
| `q` | 退出 | 终止整条链，保留已产出的成果 |

<div class="term">
<div class="term-bar">
<span class="term-dot red"></span><span class="term-dot yellow"></span><span class="term-dot green"></span>
</div>
<pre>$ ccr chain plan-impl-review.chain.yaml --input "做一个 todolist app"
▶ 段 1/3 plan [cc-switch:opus]
✓ 段 1/3 完成 (12s)
⏸  上段产出：
计划已写入 docs/plans/todolist.md
下一段：implement（profile=custom:cheap）
[回车=放行 / s=跳过 / e=改指令 / q=退出] > </pre>
</div>

::: tip `e` 改的是「下一段的指令」，不是产出文件
按 `e` 后引擎让你输入一行文本，这行会**替换下一段 prompt**，不会打开编辑器改上一段产出的文件。想改产出文件，直接 `q` 退出后手动改，或在放行前用别的终端编辑（成果就在工作目录里）。
:::

暂停的意义在于：你不是把一切交给 AI 就两手一摊了——你在关键节点检查、修正方向、确保产出符合预期。

## --auto：跳过所有暂停

如果你信任这条链、不想逐段确认，加 `--auto`：

```bash [全自动运行]
ccr chain plan-impl-review.chain.yaml --auto
```

`--auto` 会跳过**所有**放行点，链端到端跑完——**包括审查段在内，没有任何暂停**。审查段照常运行并写出 verdict，链跑完后引擎再按这个 verdict 自动决定成果合并还是保留（见下一节，fail-closed）。所以 `--auto` 不是「跳过审查」，而是「不在中途停下等你」。

::: tip 详简输出
默认输出每段会逐条打印工具调用。想更安静用 `-q`（只留段框和结果），想看思考文本和 token 用 `-v`，二者互斥。
:::

## 审查段的判定

审查段（`review: true`）运行时，引擎会在它的 prompt 末尾追加固定指令，要求 Claude 落两份产物到工作目录：

- `.ccr-chain/findings.md`：发现的问题清单（具体、可操作）
- `.ccr-chain/verdict`：单独一行，`pass` 或 `needs-work`

接下来分两种情况：

**审查段是最后一段**（最常见）——它跑完后没有放行点，链直接结束，引擎按 verdict 自动决定成果去留（见下方）。

**审查段后面还有段**（比如一个 fix 段）——会照常进入放行点，且提示里会带上判定：

```
⏸  上段产出：
审查完成，findings 见 .ccr-chain/findings.md
[判定] needs-work —— 下一段建议放行修复
下一段：fix（profile=custom:cheap）（可选，可按 s 跳过）
[回车=放行 / s=跳过 / e=改指令 / q=退出] >
```

### verdict 驱动最终的「合并 / 保留」

链跑完后，**引擎自动**根据「最后一次审查的 verdict」决定隔离区成果（`isolate: true` 时）的归宿，规则是 **fail-closed**：

- **verdict = pass** → 自动合并回当前分支
- **verdict = needs-work** → 保留成果、打印取回路径，不合并
- **没产出明确 verdict**（漏写 / 拼错）→ 同样保留、不合并——质量闸在不确定时绝不静默放行
- **整条链没有审查段** → 没有闸可拦，跑完直接合并

引擎**不会**根据 verdict 自动重跑、自动循环、自动改分支策略——它只做一次「合并还是保留」的收尾决定。中途要不要继续、跳过、改指令，仍然全在你的放行点操作里。详见 [隔离与成果交回](./isolation)。

::: tip 可选 fix 段怎么用
在链里加一个 `optional: true` 的 fix 段放在审查段之后。`optional` 本身**不会**让引擎按 verdict 自动跳过它——它只是在放行点的提示里标一句「可按 s 跳过」。实际操作是：审查给 needs-work 你就**回车放行**让 fix 跑；给 pass 你就**按 `s` 跳过** fix。决定权在你手里。
:::

## 下一步

→ [隔离与成果交回](./isolation)
