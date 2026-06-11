# ccr chain 命令行需求输入 设计

日期：2026-06-11 ｜ 分支：feat/chain

## 背景

chain（v0.3）当前给输入的唯一方式是把需求**硬写进段的 `prompt:`**。段间交棒有 `{{prev.output}}`，但第一段没有上一段可引用，所以「这次要做什么」只能写死在 yaml 里。结果是：想换需求就得改文件，yaml 无法当可复用模板。

本设计给 chain 加一个**整链级需求输入**：命令行传需求，prompt 用 `{{input}}` 占位。

## 目标 / 非目标

**目标**
- `ccr chain <file> --input "需求"` 把需求从命令行传进链。
- prompt（任意段）用 `{{input}}` 引用该需求。
- `--input` 与 `{{input}}` 对不上时 fail-fast 报错，绝不静默丢需求。

**非目标（YAGNI，先不做）**
- 多个具名变量（`{{input.foo}}`）。
- 从 stdin / 文件读需求。
- 整链级以外的变量作用域。

## CLI

```
ccr chain <file> [--auto] [--input "需求" | -i "需求"]
```
- `--input` / `-i`：其下一个参数为需求值。与 `--auto` 并列，顺序无关。
- 其余非旗标参数仍按现状当文件名（`<file>`）。
- 「是否传了 --input」由旗标是否出现决定（用 bool 记），不由值是否为空决定；允许 `--input ""`（显式空需求）。

## 占位符 `{{input}}`

- prompt 内 `{{input}}` → 替换为 `--input` 的值。
- 容忍带空格写法 `{{ input }}`，与现有 `{{prev.output}}` 一致。
- **所有段**可用（整链级需求，不限第一段）。
- 与 `{{prev.output}}` 可同段共存，各替各的 token，互不影响。
- 跑前与跑中编辑（放行点 `e`）的 prompt 走同一个 Render，故编辑文本里写 `{{input}}` 同样生效。

## Fail-fast 校验（执行任何段之前）

两个方向都报错退 1，错误信息写 stderr：

| 情况 | 处理 |
|---|---|
| 传了 `--input`，但无任何段 prompt 含 `{{input}}` | 报错：`传了 --input 但链里没任何 prompt 用 {{input}}，需求会被忽略。请在某段 prompt 里加 {{input}}。` |
| 某段 prompt 含 `{{input}}`，但没传 `--input` | 报错：`这条链有 prompt 用了 {{input}}，但没传 --input。用 ccr chain <file> --input "需求"。` |

校验在 `runChain`（CLI 层）做——只有这层同时知道「是否传了旗标」和「链内容」。

## 落点（最小改动，贴现有三层结构）

- **`internal/chain/template.go`**：`Render` 签名加 `input` 参数，多替一个 token（`{{input}}` / `{{ input }}`）。
- **`internal/chain/schema.go`**：加方法 `Chain.UsesInput() bool`——扫各段 prompt 是否含 `{{input}}` token（容忍空格）。供 CLI 校验用。
- **`internal/chain/orchestrate.go`**：`Orchestrator` 加字段 `Input string`；`Render` 调用带上 `o.Input`。
- **`internal/cli/cli.go` `runChain`**：解析 `--input`/`-i`；做上表两方向校验；把值塞进 `o.Input`。usage 串补 `--input`。
- **`internal/cli` 顶层 help**：`ccr chain` 行补一句 `--input` 说明。

## 测试

- `template_test.go`：`Render` 替 `{{input}}` / `{{ input }}`；input 与 prev.output 同段共存各替各的；无 token 时不动。
- `schema_test.go`：`UsesInput` 对含/不含 `{{input}}` 的链分别返回 true/false；容忍空格写法。
- `cli_test.go`：传 `--input` 但链不含 `{{input}}` → 退 1 且 stderr 含提示；链含 `{{input}}` 但没传 `--input` → 退 1 且 stderr 含提示；正常配对 → 解析出 input 值并注入（可借现有 orchestrator 注入式测试模式断言 prompt 被渲染）。
- `orchestrate_test.go`：设置 `Input`，断言段收到的 prompt 里 `{{input}}` 已被替换。

## 风险 / 取舍

- 严格 fail-fast 比「警告照跑」啰嗦，但符合用户偏好的「防止需求被静默吞掉」，且错误信息直接给出修复动作。
- 单变量 `{{input}}` 覆盖绝大多数「可复用模板 + 每次换需求」场景；具名多变量等真有需要再加，不预先设计。
