# ccr chain 交互式 TUI + 阶段原型 设计（v0.4.0）

日期：2026-06-15 ｜ 分支：feat/chain ｜ 目标版本：**v0.4.0**

> 状态：设计讨论已收口，待用户 review 后转实现计划（writing-plans）。

## 背景

chain（v0.3）是一条**无头串行流水线**：每段 = 一次 `claude -p`，人只在**段间放行点**入环（放行/跳过/改下段指令/退出），还能在停顿时手改 workdir 里的文件。原 spec 明确 YAGNI 了「段内对话」与「AI 生成链」。

实际用下来暴露三个痛点：

1. **无法参与规划**：规划段是无头 `claude -p`，需求只能靠 `--input` 一次性传进去——而需求一次性说全根本不现实。人想跟规划模型**多轮对话掰方向**,做不到。
2. **yaml 像黑盒**：固定串行流程又允许任意改 yaml,跑之前看不清「到底什么在驱动执行」,心里没底。
3. **太静态**：想要一个**像 Claude Code 那样、由 ccr 自己掌控的对话式终端**;有些阶段需要人真正进到里面参与,而不只在段缝里。

本设计把 chain 运行时升级成**一个 ccr 自控的常驻对话式 TUI**,把交互语义**绑定到阶段角色**,并给规划角色加**多轮对话**,同时把执行过程渲染得更好看、更透明。底层编排/安全/隔离引擎不动,主要换最外层壳并重整引擎的输入输出接口。

## 目标 / 非目标

**目标**
- ccr 自控的常驻对话式 TUI,整条链在其中运行;输出渲染做漂亮(markdown/工具调用/状态)。
- 引入**阶段原型** `plan` / `implement` / `review`,**交互语义绑定角色**:只有 `plan` 开放对话,`implement`/`review` 只看。
- `plan` 段支持**多轮对话**(ccr 自造轻量对话循环,底层走 `claude -p --resume`)。
- 开场画**整链全景面板**(治黑盒),跑中实时更新状态。
- 原型**自带默认 prompt**(经过设计),yaml 仅在需要时覆盖。
- `plan` 对话支持 `@文件` 引用、按路径附图、**剪贴板贴图(Win/Mac/Linux 三平台)**。
- schema **破坏性升级**:新增 `type` 与 `version` 字段;提供 `ccr chain migrate` 与精确报错。
- 非 TTY **降级**到纯文本前端(行为不破)。
- 引擎/前端**解耦**(事件流 + channel),引擎保持纯逻辑、可单测。
- 安全四层(隔离/白名单/黑名单钩子/路径围栏)在**对话每一轮**同样生效。

**非目标(YAGNI / 仅留口子,不进 v0.4)**
- **review 失败循环 / 分级判定**:不实现自动或人控循环;但 v0.4 必须**留好口子**(见「为未来留口」)。
- AI 自动生成链(`chain plan "目标"`)。
- 崩溃后**续跑整条链**(只保证成果与 transcript 不丢)。
- `claude` 满血对话客户端的全部能力(记忆、复杂斜杠命令等);本版只造覆盖「掰规划方向」所需的轻量循环。

## 架构:引擎 / 前端解耦

最硬的冲突:现有 `Orchestrator.Run` 是**自上而下阻塞**的 for 循环(跑段→阻塞读 stdin→下一段);Bubble Tea 是**事件循环**(Model-Update-View,内部绝不能阻塞)。两者范式相克,必须解耦。

```
            事件 (tea.Msg)
  ┌────────────┐ ───────────────▶ ┌──────────────────┐
  │  引擎       │                  │  前端(二选一)     │
  │ Orchestrator│                  │  · TUI (Bubble Tea)│
  │ (goroutine) │ ◀─────────────── │  · 纯文本 (非 TTY) │
  └────────────┘   指令 (decision  └──────────────────┘
                    / 用户输入)
```

- 编排器搬进 **goroutine**,不再直接读写终端;只**发事件**(段开始 / 工具调用 / 输出片段 / 段完成 / 到放行点 / 请求对话输入)、**收指令**(放行 / 跳过 / 退出 / 一轮对话文本 / `/done`)。
- 事件与指令各走一个 channel;前端把事件当 `tea.Msg` 渲染,把按键/输入回传。
- **引擎依旧是纯逻辑、可单测**(项目硬约定):注入 fake 段,断言「发出的事件序列」与「收到指令后的分支」,延续现有 `runSegment` 注入式测试。
- 现有 `Pauser` 接口与「直接写 `io.Writer`」退场,统一换成事件流。

## 阶段原型

链由一小撮**有名字的原型**搭成,交互与否是**角色自带属性**,不是任意可改的开关。

| 原型 | 交互 | 职责 | 默认 prompt(内置) |
|---|---|---|---|
| `plan` | **对话** | 跟人多轮聊、掰方向,产出结构化计划文件 | 「把需求拆成 阶段/任务/验证 三件套,写入 `docs/plans/<task>.md`」 |
| `implement` | 只看 | 读计划机械执行;返修段也用它(读 findings 改) | 「读计划文件,照着干,不擅自扩张」 |
| `review` | 只看 | 独立审查,写 findings + 判定 | 复用现有 `ReviewInstruction()` |

- **只有 `type: plan` 开放对话**;`implement`/`review` 永远只看。这把「危险的灵活(交互语义)」钉死,「有用的灵活(编排/挂哪个后端/几段)」全保留。
- 段的**数量与组合不固定**:`plan→implement→review`、`plan→implement→review→implement`(按 findings 返修)、甚至多个 `plan` 分阶段规划,皆可。
- **默认 prompt 内置**(经设计),yaml 可选 `prompt:` 覆盖。这让 prompt 不再是任人乱写的黑盒,顺带收紧痛点②。

## plan 段对话循环(自造轻量,走 `--resume`)

**节奏**
- 开场**自动跑第一轮**(`{{input}}` + plan 内置 prompt)→ 渲染计划初稿 → **才开放输入**。即「它先产一版,你再掰」。
- 多轮 = 一串 `claude -p --resume <session>` 调用:从 stream-json 的 init 消息**捕获 session_id**,每轮把用户消息续进同一会话,流式渲染回复,往复。

**输入**
- **多行编辑器**(bubbles/textarea)。提交键:**Enter 发送**,`Ctrl+J` 换行。
- 命令:`/done` 收工交棒、`/quit` 退出、`/retry` 重来上一轮、`/edit` 唤起 `$EDITOR` 直接改计划文件(对话掰大方向 + 手改抠细节,二者并存,不互斥)。
- `Esc` **打断当前这轮生成**但不退出对话(规划跑偏可立即叫停重说)。
- 空输入忽略,不发空轮。

**`@文件` 与图片**
- 打 `@` 触发**模糊文件选择器**(复用现有 huh fuzzy + `internal/tui`),选中把路径插入消息;那一轮 plan 模型去读它。
- **按路径附图**:`@image.png` 或拖入路径 → 那一轮用 stream-json 的 image content block 喂图。
- **剪贴板贴图(三平台)**:从系统剪贴板取图像字节 → 暂存 → 作为 image 块随该轮喂入。
  - **必须免 cgo**(项目 `CGO_ENABLED=0` 硬约束):**Windows** 用 `golang.org/x/sys/windows` syscall 调剪贴板 API;**mac** shell 出去(`pbpaste` / `osascript`);**Linux** shell 出去(`wl-paste` / `xclip`)。
  - 抽象成一个 `clipboardImage() ([]byte, error)` 接口,平台实现各一份(build tag),便于按平台测试与降级(取不到图则提示,不崩)。
- `@文件` 与附图**受路径围栏约束**:只能指 workdir 内的文件,与安全壳一致。

**交棒与校验**
- `/done` 时若**计划文件未写出**,拦一下提醒「这段没产出计划文件,确认交棒?」,别让下游接到空气。
- 交棒给下游的仍是**计划文件**(写在 workdir);`{{prev.output}}` 只给一句指路,完整对话不往下搬。

**可见性**
- 对话可滚动回看;每轮显 token / 花费 + 累计(多轮 context 越聊越贵,让人有数)。
- 对话 **transcript 落盘**到 workdir(崩溃可回看聊到哪)。

## 渲染(「更好看」的可验收口径)

- **markdown**:用 `glamour`(charmbracelet 同家),计划/回复带标题、列表、代码块高亮。
- **工具调用块**:`⏺ 读 foo.go` / `⏺ 写 plan.md`,可折叠;沿用 `-q / 默认 / -v` 三档(`-v` 展开参数与结果,`-q` 只留段标题)。
- **状态行**:当前段、profile、spinner、累计耗时;有 stream-json 的 usage 就显 **token/花费**(多后端省了多少一目了然)。
- **放行点**:`diff --stat` + 耗时,沿用现状。

## 全景面板(治黑盒)

跑前画整条链,跑中实时更新状态(✓完成 / ●运行 / ○待跑):

```
ccr chain  「plan→impl→review」  workdir: ./  隔离:on
  ① plan       [opus]      对话    把需求拆成 阶段/任务/验证
  ② implement  [haiku]     只看    读计划机械执行
  ③ review     [deepseek]  只看    独立审查 → 判定
```

原型 + 后端 + 模式 + 一句职责一眼摊开。

## Schema:从「自由段」到「原型 + 覆盖」

```yaml
version: 2            # 新增,显式标 schema 版本
name: plan-impl-review
isolate: true
segments:
  - type: plan        # 原型决定交互语义(只有 plan 对话)
    profile: opus
    # prompt: 可选覆盖,默认用原型内置模板
  - type: implement
    profile: haiku
  - type: review
    profile: deepseek
    deny: [ ... ]     # 安全开关照旧(allow_tools / deny / allow_paths)
```

**破坏性升级 + 兼容策略(a + c)**
- **a. 硬断**:只认带 `version: 2` 的新 schema;老格式给**精确报错并打印等价新写法**(可直接复制粘贴)。
- **c. `ccr chain migrate <file>`**:一次性把老 yaml 改写成新 schema(老的 `review: true`→`type: review`;裸 `prompt` 段→`type: implement`)。
- **不养双解析**:加载层只认新格式,迁移交给 migrate 命令,避免长期双分支包袱。
- 版本检测靠 `version` 字段(缺失 = 老格式 → 报错指路 migrate)。

## `--auto`

`plan` 段没法停着等人 → 退化成**只跑开场一轮、不开输入**(临时当只看段);判定照写不分叉,沿用现状。非 TTY 等同 `--auto`(详见下)。

## 非 TTY 降级

- Bubble Tea 要 TTY;CI / 管道 / 重定向时**退回纯文本前端**(现有行式流式渲染),与 TUI 前端**共用同一引擎事件流**——这反证「引擎/前端解耦」的必要。
- 非 TTY 下 `plan` 段无法交互,等同 `--auto`。

## 安全(全程复用,含对话每轮)

- 隔离三态收尾、`allow_tools` 白名单、`PreToolUse` 黑名单钩子、路径围栏——引擎一律不动。
- **关键**:每次 `--resume` 都带**同样的 settings(黑名单钩子)+ 路径围栏 env**,交互轮与一次性段共用一套壳,绝不让多轮成为安全旁路。
- `@文件` / 附图同样圈在 workdir 内。

## 为未来留口(forward-compat 约束,不实现但不堵死)

review 失败循环 / 分级判定不进 v0.4,但 v0.4 的实现**必须满足**:

1. **编排循环允许「跳回第 K 段」**:事件流引擎别把「段索引只能 +1」写死;接口先留(哪怕 v0.4 永远只 +1)。
2. **判定字段可扩展**:别把 `pass/needs-work` 二元焊死在类型里,将来加 severity(`minor/serious/blocker`)不破坏存量。
3. 隔离收尾已按「最后一次审查判定」(`lastReviewNeedsWork`)走,本就为多轮审查留了余地,保持。

> 未来形态(仅记录,不做):判定升级为分级 + **人控有界循环**(每轮人批 + 轮次计数 + 上限),且循环决策落在交互 `plan` 段上(findings 当 `{{input}}` 喂回),天然复用 v0.4 的交互能力。绝不做无人值守自动循环(防成本失控 / 死循环)。

## 测试

- **引擎(事件流)**:纯逻辑单测——注入 fake 段,断言事件序列与收指令后的分支。
- **多轮 `--resume`**:helper-process 模式集成测(项目已有这套)。
- **剪贴板贴图**:`clipboardImage()` 接口按平台实现 + 各自测试;取不到图的降级路径要测。
- **TUI**:`charmbracelet/x/exp/teatest` 做交互/golden,或退一步只测纯文本前端 + TUI 手验。

## 技术栈 / 约束

- 新增:Bubble Tea(事件循环)、bubbles/textarea(多行输入)、glamour(markdown);复用现有 lipgloss(`internal/ui`)、huh fuzzy(`internal/tui`)。
- **硬约束:`CGO_ENABLED=0`**——剪贴板取图走免 cgo 路线(见上)。

## 落点(模块)

- `internal/chain/orchestrate.go`:`Run` 改为事件流 + channel;循环允许跳回段(接口)。
- `internal/chain/`(新):对话会话循环(session_id 捕获 + `--resume` 多轮)、事件类型定义。
- `internal/chain/schema.go`:`version` + `type` 字段;原型内置默认 prompt;加载层只认新格式 + 精确报错。
- `internal/chain/templates`:原型模板更新。
- 新前端包(如 `internal/chaintui`):Bubble Tea TUI(全景面板 / 流式渲染 / 对话编辑器 / `@文件` 选择器);纯文本前端。
- 新剪贴板包(如 `internal/clipboard`):`clipboardImage()` + 平台实现(build tag)。
- `internal/cli/cli.go`:`runChain` 接事件流前端;新增 `ccr chain migrate`;usage/help 更新。

## 分期(v0.4 内部 track,降风险)

1. **Track 1 — 引擎/前端解耦**:`Run` 改事件流 + channel,纯文本前端先顶,**行为不变**,可单独合。
2. **Track 2 — 原型 schema + 全景面板 + 漂亮渲染(只看段)**:看得见的第一波收益;`plan` 此时仍一次性。含 `version`/`type`/migrate。
3. **Track 3 — plan 多轮对话**:压轴,风险最高;含 `--resume` 循环、`@文件`、附图、三平台剪贴板贴图。

## 风险 / 取舍

- **最大成本**:Track 3 等于造一个轻量对话客户端,工程量与体验风险最高,故放最后、范围卡死在「掰规划方向所需」。
- **跨平台剪贴板贴图**:免 cgo 约束下三平台各一套实现,是单项最重的活;抽象成接口 + 降级兜底。
- **破坏性 schema**:用户少、可接受;靠 migrate + 报错打印新写法把迁移成本压到近零。
- **TUI 测试**:比纯逻辑难;靠「引擎纯逻辑可单测 + TUI 用 teatest/手验」分而治之。
