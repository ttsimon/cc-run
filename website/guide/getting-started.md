# 快速上手

装好 `ccr` 之后，三步走。

## 1. 看看有哪些配置

```bash
$ ccr ls
```

<div class="term">
<div class="term-bar">
<span class="term-dot red"></span><span class="term-dot yellow"></span><span class="term-dot green"></span>
</div>
<pre>$ ccr ls
● kimi       [cc-switch] sonnet  https://api.moonshot.cn/anthropic
  deepseek   [cc-switch] sonnet  https://api.deepseek.com/anthropic
  glm        [custom   ]         https://open.bigmodel.cn/api/anthropic</pre>
</div>

每行格式是 `名字 [来源] 模型 baseURL`。`[cc-switch]` 表示来自 cc-switch 数据库（只读），`[custom]` 表示来自自定义目录。行首的 `●` 标记 cc-switch 当前的全局 provider。

## 2. 选一个启动

```bash
$ ccr                # 不传参 → 交互式选择
$ ccr deepseek       # 精确名字直启
$ ccr .              # 默认配置（需要先设过）
$ ccr -              # 重跑上次用的
$ ccr de             # 模糊命中：唯一则直启，多个则弹选择器
```

<div class="term">
<div class="term-bar">
<span class="term-dot red"></span><span class="term-dot yellow"></span><span class="term-dot green"></span>
</div>
<pre>$ ccr deepseek
╭───────────────────────────╮
│ ✻ Welcome to Claude Code! │
╰───────────────────────────╯
  ...</pre>
</div>

CC Run 会给当前终端注入 deepseek 的 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN`，然后直接 `exec` 拉起 `claude`——CC Run 本身不打印任何提示，claude 的界面直接接管终端。你在 Claude Code 里什么也不用改。

## 3. 多开

再开一个终端 tab，跑另一个名字：

```bash
$ ccr kimi
```

第三个 tab 跑第三个：

```bash
$ ccr glm
```

三个终端、三个后端，各自独立，互不干扰。这就是 CC Run 的核心用法。

## 透传 Claude Code 参数

`ccr` 后面的额外参数会原样透传给 `claude`：

```bash
$ ccr deepseek --model haiku        # 选 deepseek 后端 + 指定 haiku 模型
$ ccr kimi -p "你好"                 # 选 kimi 后端 + 带 prompt 直接对话
```

## 下一步

→ [配置与按名解析](./profiles)
