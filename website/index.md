---
layout: home
hero:
  name: ccr
  text: 给每个终端注入不同后端，再拉起 claude
  tagline: 同时多开、各用不同后端、互不干扰，且不改全局配置。
  actions:
    - theme: brand
      text: 快速上手
      link: /guide/getting-started
    - theme: alt
      text: 为什么是 ccr
      link: /guide/what-is-ccr
    - theme: alt
      text: GitHub
      link: https://github.com/ttsimon/cc-run
features:
  - icon: 🪟
    title: 多开互不干扰
    details: 每个终端各跑一次 ccr &lt;名&gt;，各用各的后端，并行不打架。
  - icon: 🔌
    title: 不改全局配置
    details: 按终端会话注入 env——cc-switch 切的是全局，ccr 切的是这个 tab。
  - icon: ⛓️
    title: chain 多后端流水线
    details: 规划用强模型、实现用便宜模型、审查换一家，串起来跑。
  - icon: 📦
    title: 跨平台单二进制
    details: Windows / macOS / Linux，Scoop / Homebrew / go install 都行。
---

<div class="term">
<div class="term-bar">
<span class="term-dot red"></span><span class="term-dot yellow"></span><span class="term-dot green"></span>
</div>
<pre>$ ccr ls
kimi        (ccswitch)  默认 ·
deepseek    (ccswitch)
glm         (custom)    别名: g
$ ccr deepseek            # 给当前终端注入 deepseek 后端再拉起 claude
→ launching claude with provider: deepseek</pre>
</div>
