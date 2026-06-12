# 概念

**chain 是 CC Run 的多后端流水线功能**——把一次完整任务拆成多个段（segment），每段用不同 provider 拉起 `claude -p` 执行，段与段之间通过文件和占位符传递上下文。

## 它填补了什么空白

官方子代理（sub-agent）和 Agent SDK 能做的事情很多，但它们有一个共同的盲区：**异构后端流水线**。

现实需求很简单：用户想把不同模型串起来用——规划用强模型、实现用便宜模型、审查换一家独立检查——但目前没有工具支持这种"一段一个后端"的编排。你只能端到端用一个模型跑到底。

CC Run chain 就是为这个空白而设计的。

## 参考链：三段式流水线

chain 内置一个参考模板，开箱即用：

```
Plan（强模型） → Implement（便宜模型） → Review（另一家 provider）
```

**设计哲学**：智力前置到规划阶段。强模型把计划拆得很细——分阶段、分任务、写验收标准——到了实现阶段，就变成"照计划干活"的工作，便宜模型完全能胜任。最后，由第三家独立审查，捕捉问题和遗漏。

::: tip 为什么不用同一个模型审查自己？
自己审查自己容易漏掉盲点。换一家 provider 来做 review，就像代码 review 找同事而不是自己审——多一双不同的"眼睛"，更容易发现问题。
:::

## 使用方式

两条命令：

```bash [初始化模板]
ccr chain init            # 在当前目录生成 chain.yaml 模板
ccr chain init mychain    # 生成 mychain.yaml
```

```bash [运行链]
ccr chain chain.yaml                     # 运行
ccr chain chain.yaml --input "做一个 todolist app"   # 注入需求
ccr chain chain.yaml --auto               # 跳过所有暂停，端到端跑完
```

模板文件会带着完整的注释和字段说明，把它当起点来改，比自己从零写快得多。

## 引擎如何工作

三段式引擎，每段职责单一：

1. **分段执行**：每个 segment 是一个隔离的 `claude -p` 调用，执行完就退出——天然就是 handoff 边界。每段的 provider 由 yaml 中的 `profile` 字段指定。
2. **交接（handoff）**：所有 segment 共享同一个工作目录，文件天然持久化。前一段的产出写入约定路径（如 `docs/plans/<task>.md`），后一段通过 <span v-pre>`{{prev.output}}`</span> 占位符得到方向性指引。
3. **人在回路中（human in the loop）**：默认每段结束后暂停，给你检查、编辑、跳过或退出的机会。详情见 [放行与审查](./pausing)。

## 下一步

→ [写一条链](./writing-a-chain)
