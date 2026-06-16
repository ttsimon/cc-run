# 写一条链

链的定义就是一个 YAML 文件。最直接的方式是 `ccr chain init` 生成模板然后改。

## 完整示例

<div v-pre>

```yaml [chain.yaml]
name: plan-impl-review
isolate: true
segments:
  - name: plan
    profile: strong
    prompt: |
      为「{{input}}」写详细实现计划。
      拆出阶段、每个阶段拆任务、每个任务写验收标准。
      把完整计划写入 docs/plans/ 目录。
  - name: impl
    profile: cheap
    prompt: |
      上一段的产出在 {{prev.output}}。
      请阅读计划文件，然后逐一实现。
  - name: review
    profile: another
    prompt: |
      审查上一段实现的成果，对照计划逐项检查。
      把发现和判定写入 findings，最后给出 verdict（pass / needs-work）。
    review: true
  - name: fix
    profile: cheap
    prompt: |
      上一段的 findings 在 {{prev.output}}。
      对照修复。
    optional: true
```

</div>

::: tip profile 字段的值
`profile` 必须是 CC Run 已知的 provider 名字——也就是 `ccr ls` 能列出来的名字，支持精确匹配、别名和模糊命中。详见 [配置与按名解析](../guide/profiles)。
:::

## 字段参考

### Chain 顶层

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 链的名字，用于显示和日志 |
| `isolate` | boolean | 否 | 是否隔离到临时工作区（**默认 `false`**——省略即不隔离，直接在 `workdir` 操作）。要隔离必须显式写 `isolate: true` |
| `workdir` | string | 否 | 指定工作目录，默认当前目录 |
| `segments` | array | 是 | 段列表，按顺序执行 |

::: warning isolate 默认不开
`isolate` 缺省为 `false`，链会**直接在你的工作目录里跑**。需要可回滚的隔离区（见 [隔离与成果交回](./isolation)），必须显式写 `isolate: true`——内置模板已经替你写好了。
:::

### Segment

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 段名，用于显示和日志 |
| `profile` | string | 是 | CC Run provider 名字，决定本段用哪个后端 |
| `prompt` | string | 是 | 发给 claude 的 prompt，可含占位符 |
| `allow_tools` | string[] | 否 | 本段允许的工具白名单（claude `--allowedTools`） |
| `deny_commands` | string[] | 否 | 本段额外禁用的 shell 命令（叠加到内置默认） |
| `allow_paths` | string[] | 否 | 路径围栏的白名单逃生口（如 `["/tmp"]`）。默认本段只能读写 `workdir` 内的路径；列在这里的路径额外放行 |
| `review` | boolean | 否 | 是否为审查段（默认 `false`） |
| `optional` | boolean | 否 | 是否为可选段（默认 `false`）。仅影响 `⏸` 提示里的「可跳过」提示文案；任何段在放行点都能按 `s` 跳过 |

## 占位符

prompt 中可以使用两个占位符，运行时自动替换：

| 占位符 | 替换内容 | 来源 |
|--------|----------|------|
| <code v-pre>{{input}}</code> | 链的需求描述 | `ccr chain <file> --input "需求"` |
| <code v-pre>{{prev.output}}</code> | 上一段的产出路径 | 引擎自动设置为上一段写入的约定路径 |

::: warning 注意
<code v-pre>{{prev.output}}</code> 给出的是**方向性的指引**（比如"计划文件在 docs/plans/xxx.md，去读它然后干活"），不是上一段的完整输出内容。引擎只传递路径，不传递大段文本。segment 自己去读文件。
:::

## 审查段

设置 `review: true` 标记审查段。引擎会在审查段 prompt 末尾自动追加固定指令，要求 Claude：

- 把发现的问题清单写到 `.ccr-chain/findings.md`
- 在 `.ccr-chain/verdict` 写单独一行：`pass` 或 `needs-work`

这个 `verdict` **不只是给人看的**——链跑完后，引擎会根据「最后一次审查的 verdict」自动决定隔离区成果是合并回去还是保留（fail-closed：只有明确 `pass` 才自动合并）。详见 [隔离与成果交回](./isolation)。引擎不会据 verdict 自动重跑或循环，但它确实驱动最终的「合并 / 保留」决策。

详情见 [放行与审查](./pausing)。

## 下一步

→ [放行与审查](./pausing)
