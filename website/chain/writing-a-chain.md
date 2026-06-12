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
`profile` 必须是 CC RUN 已知的 provider 名字——也就是 `ccr ls` 能列出来的名字，支持精确匹配、别名和模糊命中。详见 [配置与按名解析](../guide/profiles)。
:::

## 字段参考

### Chain 顶层

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 链的名字，用于显示和日志 |
| `isolate` | boolean | 否 | 是否隔离到临时工作区（默认 `true`） |
| `workdir` | string | 否 | 指定工作目录，默认当前目录 |
| `segments` | array | 是 | 段列表，按顺序执行 |

### Segment

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 段名，用于显示和日志 |
| `profile` | string | 是 | CC RUN provider 名字，决定本段用哪个后端 |
| `prompt` | string | 是 | 发给 claude 的 prompt，可含占位符 |
| `allow_tools` | string[] | 否 | 本段允许的工具白名单 |
| `deny_commands` | string[] | 否 | 本段额外禁用的 shell 命令（叠加到内置默认） |
| `review` | boolean | 否 | 是否为审查段（默认 `false`） |
| `optional` | boolean | 否 | 是否为可选段（默认 `false`），可选段可在 `⏸` 时跳过 |

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

设置 `review: true` 标记审查段。审查段有特殊行为：

- Claude 应写出 `findings`（发现的问题）和 `verdict`（`pass` 或 `needs-work`）
- 审查段结束时一定暂停（即使在 `--auto` 模式下），让用户查看结果
- `verdict` 是给人看的，引擎不根据它自动分支

详情见 [放行与审查](./pausing)。

## 下一步

→ [放行与审查](./pausing)
