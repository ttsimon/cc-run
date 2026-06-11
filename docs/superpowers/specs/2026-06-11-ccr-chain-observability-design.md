# ccr chain 执行可观测性与输出样式 设计（track B）

日期：2026-06-11 ｜ 分支：feat/chain ｜ 范围：v0.3 / chain 发版前

## 动机

现在 chain 跑起来是个黑箱：段以 `claude -p ... --output-format text` 无头跑，ccr 把 stdout 整个吞进 `bytes.Buffer`（为了当 `{{prev.output}}` 交棒），既不回显过程、也不回显结果。非 auto 的放行点能看到的也只有上段文本 + 下段信息 + 菜单。用户全程看不到「agent 在干什么、卡在哪、改了哪些文件」。

本设计让 chain 执行**看得见过程**，并给整个 CLI 工具一层**统一、好看**的终端输出样式。

## 目标 / 非目标

**目标**
- 段执行实时可见：逐个工具调用、最终结果，按详细度可调。
- 详细度可切换：`-q` / 默认 / `-v` 三级。
- 放行点信息加厚：本段改了哪些文件、审查 verdict、耗时。
- 共享样式层：chain 先用，工具其余命令可逐步采纳，风格统一、好看。
- 非 TTY（管道/重定向/CI）自动降级：无转圈、无颜色控制符。

**非目标（YAGNI）**
- 把详细度做成 yaml 字段（它是运行时偏好，CLI 旗标即可）。
- 一次性重排所有命令（ls/show/doctor…）的输出——样式层先建好、chain 先用，其余命令的采纳作为后续增量。
- 成本/计费的精确核算（verbose 显 token 用量即可，不做账单）。

## 架构：一条 stream-json 流 + 详细度过滤

核心决策：**段一律用 `--output-format stream-json` 跑，ccr 解析这一条事件流；详细度只是渲染过滤器**，不为不同级别开不同的 claude 输出格式。

```
claude -p ... --output-format stream-json --verbose
        │  (stdout: 一行一个 JSON 事件)
        ▼
   eventParser  ── 防脆解析，未知事件优雅降级
        │  []Event
        ▼
   renderer(level, tty) ── 按 quiet/normal/verbose 决定打多少；
        │                   非 TTY 关转圈/颜色
        ├─► 终端（实时）
        └─► 抽取最终 result 文本 ──► {{prev.output}} 交棒
```

好处：集成边界（stream-json schema）只踩一处、三级共用；交棒输出永远从「最终 result 事件」抽，与级别无关、稳定；加/改级别只动渲染。

## 组件

### 1. 段输出格式切换
`SegmentArgs` 里 `--output-format text` 改为 `--output-format stream-json --verbose`（`-p` 模式下 stream-json 需配 `--verbose` 才逐事件输出）。这是代码里既有「⚠️ 集成边界」注释覆盖的同一处，旗标变更集中在此。

### 2. eventParser（`internal/chain/stream.go`，新）
- 逐行读 claude stdout，每行 `json.Unmarshal` 成一个事件（按 `type` 字段分派：system/assistant/user(tool_result)/result）。
- **防脆**：未知 `type`、解析失败的行 → 不报错中断，降级为「原样留存 / 忽略」，最多少显细节，绝不让解析失败弄崩整段或致盲。
- 输出统一的内部事件类型（段无关），供 renderer 消费；并标出哪条是「最终 result」（含 result 文本 + usage）。

### 3. renderer（`internal/chain/render.go`，新）
按 `level` 决定渲染哪些事件：

| 级别（旗标） | 渲染内容 |
|---|---|
| **quiet** (`-q`) | 段框 + 从流抽出的最终结果文本 |
| **normal**（默认） | + 逐个 `tool_use` 行（Read/Write/Edit/Bash + 目标） |
| **verbose** (`-v`) | + assistant 思考文本、token 用量 |

- 段框始终打（连 quiet 也有）：`▶ 段 2/3 implement [DeepSeek]` 开头、`✔ 段 2/3 完成 (0:58)` 结尾（含耗时、退出状态）。
- 运行中转圈 + 计时（仅 TTY）。
- 工具行示例：`🔧 Write web/index.html`、`🔧 Bash git add -A && git commit`。

### 4. 交棒抽取
`{{prev.output}}` 的值从「最终 result 事件」的 result 文本取（替代现在「捕获整个 stdout」）。renderer 与捕获解耦：renderer 负责显示，parser 负责把 result 文本交给 orchestrator 当 `prev`。

### 5. 放行点加厚（`pause` 展示，与详细度无关）
非 auto 停顿时，除现有「上段输出 + 下段信息 + 菜单」，补：
- **本段改了哪些文件**：隔离区里 `git diff --stat`（复用 track A 的 worktree；copydir 模式用快照比对的文件名列表）。
- **审查 verdict**（已有数据，强化展示）。
- **本段耗时**。

### 6. 样式层（`internal/ui/style.go`，新）
- 用 **lipgloss**（已在依赖树，提为直接依赖）定义一套调色板 + 符号集 + 样式（段框、工具行、成功/失败、judgement、放行菜单）。
- **TTY / 颜色降级**：用 `mattn/go-isatty`（已在树）判定 stdout 是否 TTY；非 TTY → 关转圈、lipgloss 走无色（termenv/colorprofile 自动尊重 `NO_COLOR` 与无色环境）。
- 作为**共享层**：chain 先消费；ls/show/doctor 等后续可改用同一套 style，达成全工具统一。本 spec 只要求建层 + chain 用上，不强制改其余命令。

## CLI

```
ccr chain <file> [--auto] [--input "需求"] [-q | -v]
```
- 不带 → normal；`-q` → quiet；`-v` → verbose。
- 与 `--auto`、`--input` 正交，顺序无关。
- 互斥校验：同时给 `-q -v` → 报错退 1。

## 落点

- `internal/chain/runner.go`：`SegmentArgs` 改 stream-json；`RunSegment` 把 stdout 喂给 parser（而非裸 buffer），实时渲染 + 抽取 result。
- `internal/chain/stream.go`（新）：eventParser。
- `internal/chain/render.go`（新）：renderer（level + tty 感知）。
- `internal/chain/orchestrate.go`：传递 level；放行点加厚（diff --stat / verdict / 耗时）。
- `internal/chain/pause.go`：放行点展示用样式层 + 新增信息。
- `internal/ui/style.go`（新）：共享样式层。
- `internal/cli/cli.go`：解析 `-q`/`-v`、互斥校验、help 行。

## 测试

- `eventParser`：正常事件流解析出工具调用/最终 result；**未知 type / 坏 JSON 行优雅降级**不报错；抽取 result 文本正确。
- `renderer`：quiet/normal/verbose 各渲染对应子集（用假事件流断言输出含/不含工具行、思考行）；非 TTY 下无 ANSI/转圈。
- 交棒：从 stream 抽出的 result 文本 = 传给下一段的 `prev`（替代 buffer 捕获后回归测试不变）。
- `pause` 加厚：放行点文本含 diff --stat / verdict / 耗时。
- CLI：`-q -v` 同给报错；`-q`/`-v` 正确映射级别。

## 风险 / 集成边界

- **stream-json schema 是集成边界**：事件结构按当前 Claude Code 写，本地未用真 claude 验过。防脆解析是主要缓解——schema 漂移最多少显细节，不致盲不致崩。实现时跑一次真 `claude -p --output-format stream-json --verbose` 校准事件结构，差异只改 `stream.go` 的解析。
- 交棒来源从「stdout 全量」变为「result 事件文本」，语义更准（只取最终答案，不含过程噪声），但需回归确认下一段拿到的 `prev` 与预期一致。
- 与 track A 弱耦合：放行点的 `git diff --stat` 依赖 track A 的隔离工作区；非隔离时退化为对当前目录跑 diff（或省略）。
