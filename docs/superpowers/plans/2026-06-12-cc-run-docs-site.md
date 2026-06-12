# cc-run 文档站（VitePress）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
> **提交约定**：Conventional Commits（`docs:`/`build:`/`ci:`/`feat:`）；每条 commit 末尾加 `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`。本工程为前端文档站，**无 Go `task check`**；正确性闸是 `npm run docs:build`（VitePress 默认死链即报错）。

**Goal:** 在同仓库 `website/` 下建一个 VitePress 多页文档站（中文优先、i18n 友好、本地搜索、暗色品牌主题），首页为落地页，GitHub Actions 自动部署到 GitHub Pages。

**Architecture:** 顶层独立 `website/` VitePress 工程，与内部 `docs/superpowers/` 物理隔离。`.vitepress/config.ts` 配 base/i18n/nav/sidebar/搜索；`theme/custom.css` 覆盖 VitePress CSS 变量落地品牌色板；内容分 `guide/`·`chain/`·`reference/` 三区 + `index.md` 落地首页；`.github/workflows/docs.yml` 构建并部署 Pages。

**Tech Stack:** VitePress 1.x、Node 20 LTS、npm、GitHub Pages（Actions 部署）。spec：`docs/superpowers/specs/2026-06-12-cc-run-docs-site-design.md`（含像素级视觉规格——本计划凡涉及样式处以该 spec 为准）。

**内容来源（写页面时据此提炼，勿原样照抄全文）：** `README.md`、`CLAUDE.md`、`docs/superpowers/specs/2026-06-09-ccr-chain-design.md`、`docs/superpowers/specs/2026-06-11-ccr-chain-isolation-design.md`、`internal/chain/schema.go`（chain yaml 字段）、`internal/cli/cli.go` 的 `printUsage`（命令清单）。

---

## File Structure

```
website/
  package.json                 # vitepress devDep + scripts
  .gitignore                   # node_modules / dist / cache
  index.md                     # 落地首页（hero + features + 终端卡）
  guide/{what-is-ccr,installation,getting-started,profiles,doctor}.md
  chain/{concept,writing-a-chain,pausing,isolation,security}.md
  reference/{commands,files,faq}.md
  .vitepress/
    config.ts                  # 站点配置
    theme/index.ts             # extends 默认主题 + 引 custom.css
    theme/custom.css           # 品牌色板 + 终端卡 CSS
.github/workflows/docs.yml     # 构建 + 部署 Pages
```

每个文件单一职责：`config.ts` 只管站点结构与导航；`custom.css` 只管视觉 token；内容页只管内容。

---

## Task 1: 脚手架 VitePress 工程

建 `website/` 工程骨架，能本地 `dev`/`build` 跑起来（先用占位首页）。

**Files:**
- Create: `website/package.json`
- Create: `website/.gitignore`
- Create: `website/index.md`（占位，Task 3 替换为正式落地页）

- [ ] **Step 1: 写 package.json**

`website/package.json`：
```json
{
  "name": "cc-run-docs",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "docs:dev": "vitepress dev",
    "docs:build": "vitepress build",
    "docs:preview": "vitepress preview"
  },
  "devDependencies": {
    "vitepress": "^1.6.3"
  }
}
```

- [ ] **Step 2: 写 .gitignore**

`website/.gitignore`：
```
node_modules/
.vitepress/dist/
.vitepress/cache/
```

- [ ] **Step 3: 写占位首页**

`website/index.md`：
```markdown
# ccr

占位首页，Task 3 替换为正式落地页。
```

- [ ] **Step 4: 安装并验证 dev/build 跑通**

Run（在 `website/` 目录）：
```bash
npm install
npm run docs:build
```
Expected: `npm install` 生成 `node_modules` 与 `package-lock.json`；`docs:build` 成功，输出 `.vitepress/dist`，无报错。
（`vitepress dev` 为交互式，本步只验证 `build`。）

- [ ] **Step 5: 提交**

```bash
git add website/package.json website/.gitignore website/index.md website/package-lock.json
git commit -m "build: scaffold VitePress docs project under website/"
```

---

## Task 2: 主题与站点配置

落地品牌视觉 + 站点结构（base/i18n/nav/sidebar/搜索）。

**Files:**
- Create: `website/.vitepress/config.ts`
- Create: `website/.vitepress/theme/index.ts`
- Create: `website/.vitepress/theme/custom.css`

- [ ] **Step 1: 写 theme/custom.css（品牌色板 + 终端卡）**

`website/.vitepress/theme/custom.css` —— 完整内容取自 spec「视觉与样式设计 / 设计 token」与「终端卡」两节，逐字落地：
```css
/* ── 暗色（主打）── */
.dark {
  --vp-c-bg:        #0c0f17;
  --vp-c-bg-alt:    #0a0d14;
  --vp-c-bg-soft:   #121826;
  --vp-c-bg-elv:    #161d2e;
  --vp-c-text-1:    #e6ebf5;
  --vp-c-text-2:    #94a3b8;
  --vp-c-text-3:    #64748b;
  --vp-c-divider:   #243049;
  --vp-c-border:    #243049;
  --vp-c-gutter:    #0a0d14;
  --vp-c-brand-1:   #8ea8ff;
  --vp-c-brand-2:   #7c9cff;
  --vp-c-brand-3:   #5b7cf0;
  --vp-c-brand-soft: rgba(124,156,255,0.14);
  --ccr-accent:     #ffb86b;
  --ccr-grad:       linear-gradient(120deg, #7c9cff 0%, #56e1c4 100%);
}

/* ── 明色（仅换品牌色）── */
:root {
  --vp-c-brand-1:   #4c6ef0;
  --vp-c-brand-2:   #5b7cf0;
  --vp-c-brand-3:   #7c9cff;
  --vp-c-brand-soft: rgba(92,124,240,0.12);
  --ccr-accent:     #d97706;
  --ccr-grad:       linear-gradient(120deg, #5b7cf0 0%, #14b8a6 100%);

  --vp-font-family-base: 'Inter','Segoe UI',system-ui,-apple-system,'Microsoft YaHei',sans-serif;
  --vp-font-family-mono: 'JetBrains Mono','Cascadia Code',Consolas,monospace;

  --vp-home-hero-name-color: transparent;
  --vp-home-hero-name-background: var(--ccr-grad);

  --vp-button-brand-bg: var(--vp-c-brand-3);
  --vp-button-brand-hover-bg: var(--vp-c-brand-2);
  --vp-button-brand-active-bg: var(--vp-c-brand-3);
  --vp-button-brand-text: #ffffff;
}

/* feature 卡：大圆角 + hover 抬升转品牌边 */
.VPFeature { border-radius: 14px; transition: transform .15s, border-color .15s; }
.VPFeature:hover { transform: translateY(-2px); border-color: var(--vp-c-brand-2); }

/* 终端卡 */
.term { background: var(--vp-c-bg-soft); border:1px solid var(--vp-c-border);
        border-radius:14px; overflow:hidden; margin: 24px 0; }
.term-bar { display:flex; gap:6px; padding:12px 16px;
            background:rgba(0,0,0,.2); border-bottom:1px solid var(--vp-c-border); }
.term-dot { width:12px; height:12px; border-radius:50%; }
.term-dot.red{background:#ff5f57}.term-dot.yellow{background:#febc2e}.term-dot.green{background:#28c840}
.term pre { margin:0; padding:18px 20px; font-family:var(--vp-font-family-mono);
            font-size:.85rem; line-height:1.7; overflow-x:auto; white-space:pre; }

@media (prefers-reduced-motion: reduce) {
  .VPFeature, .VPFeature:hover { transition: none; transform: none; }
}
```

- [ ] **Step 2: 写 theme/index.ts**

`website/.vitepress/theme/index.ts`：
```ts
import DefaultTheme from 'vitepress/theme'
import './custom.css'

export default DefaultTheme
```

- [ ] **Step 3: 写 config.ts（base/i18n/nav/sidebar/搜索）**

`website/.vitepress/config.ts`：
```ts
import { defineConfig } from 'vitepress'

export default defineConfig({
  base: '/cc-run/',
  lang: 'zh-CN',
  title: 'ccr',
  description: '用选定 provider 的环境变量启动 claude——多开各用不同后端、互不干扰、不改全局配置。',
  lastUpdated: true,
  cleanUrls: true,

  // i18n：root = 简体中文（挂 /，无前缀）。将来加英文：在此加 en locale + website/en/。
  locales: {
    root: { label: '简体中文', lang: 'zh-CN' }
    // en: { label: 'English', lang: 'en', link: '/en/' }  // 预留，本期不写
  },

  themeConfig: {
    nav: [
      { text: '指南', link: '/guide/what-is-ccr' },
      { text: 'chain', link: '/chain/concept' },
      { text: '参考', link: '/reference/commands' }
    ],
    sidebar: {
      '/guide/': [
        {
          text: '指南',
          items: [
            { text: '这是什么', link: '/guide/what-is-ccr' },
            { text: '安装', link: '/guide/installation' },
            { text: '快速上手', link: '/guide/getting-started' },
            { text: '配置与按名解析', link: '/guide/profiles' },
            { text: 'doctor 体检', link: '/guide/doctor' }
          ]
        }
      ],
      '/chain/': [
        {
          text: 'chain 多后端流水线',
          items: [
            { text: '概念', link: '/chain/concept' },
            { text: '写一条链', link: '/chain/writing-a-chain' },
            { text: '放行与审查', link: '/chain/pausing' },
            { text: '隔离与成果交回', link: '/chain/isolation' },
            { text: '安全', link: '/chain/security' }
          ]
        }
      ],
      '/reference/': [
        {
          text: '参考',
          items: [
            { text: '命令速查', link: '/reference/commands' },
            { text: '配置文件与路径', link: '/reference/files' },
            { text: 'FAQ / 故障排查', link: '/reference/faq' }
          ]
        }
      ]
    },
    search: {
      provider: 'local',
      options: {
        // 中文可搜：放宽分词，按单字切分兜底
        miniSearch: {
          options: {
            tokenize: (text: string) => text.split(/[\s\-/]+|(?<=[一-龥])(?=[一-龥])/)
          }
        }
      }
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/ttsimon/cc-run' }
    ],
    editLink: {
      pattern: 'https://github.com/ttsimon/cc-run/edit/master/website/:path',
      text: '在 GitHub 上编辑此页'
    },
    docFooter: { prev: '上一页', next: '下一页' },
    outline: { label: '本页目录' },
    lastUpdatedText: '最后更新'
  }
})
```

- [ ] **Step 4: 验证构建**

Run（`website/`）：`npm run docs:build`
Expected: 成功。占位首页 + 空的导航/侧边栏链接此刻指向尚不存在的页面——**VitePress 死链检查会报错**。因此本步**仅验证 config 语法不崩**：临时把 `nav`/`sidebar` 指向已存在的 `index.md`，确认 build 过后改回；或接受本步 build 失败于死链、待 Task 3-6 页面补齐后在 Task 7 整体验证。**采用后者**：本步只需 `npx vitepress build` 能进入构建（解析 config 成功），死链报错可接受。

> 说明：为避免反复改 config，本任务的 build 仅确认配置被正确解析（无 TS/语法错误）。完整死链通过在 Task 7 所有页面就位后验证。

- [ ] **Step 5: 提交**

```bash
git add website/.vitepress/config.ts website/.vitepress/theme/index.ts website/.vitepress/theme/custom.css
git commit -m "feat: VitePress config and brand theme (dark palette, i18n root, local search)"
```

---

## Task 3: 落地首页

正式 `index.md`：hero + 4 张 feature 卡 + 终端卡主视觉。

**Files:**
- Modify: `website/index.md`（替换 Task 1 占位）

- [ ] **Step 1: 写正式首页**

`website/index.md` —— frontmatter 与 features 文案逐字取自 spec「组件规格 / Hero / Feature 卡」：
```markdown
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
    details: 每个终端各跑一次 ccr <名>，各用各的后端，并行不打架。
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
    <span class="term-dot red"></span>
    <span class="term-dot yellow"></span>
    <span class="term-dot green"></span>
  </div>
  <pre>$ ccr ls
  kimi        (ccswitch)  默认 ·
  deepseek    (ccswitch)
  glm         (custom)    别名: g

$ ccr deepseek            # 给当前终端注入 deepseek 后端再拉起 claude
→ launching claude with provider: deepseek</pre>
</div>
```

- [ ] **Step 2: 验证构建（首页可独立 build）**

Run（`website/`）：`npx vitepress build`
Expected: 首页构建成功。若因 nav/sidebar 指向未建页面而死链报错，属预期（Task 7 整体验证）；本步确认首页 frontmatter/HTML 无语法错误即可。

- [ ] **Step 3: 提交**

```bash
git add website/index.md
git commit -m "docs: home landing page with hero, feature cards and terminal demo"
```

---

## Task 4: 指南五页（guide/）

**Files:**
- Create: `website/guide/what-is-ccr.md`
- Create: `website/guide/installation.md`
- Create: `website/guide/getting-started.md`
- Create: `website/guide/profiles.md`
- Create: `website/guide/doctor.md`

> 写作要求：每页首行 `# 标题`；中文；命令用带 `[标题]` 的代码块；演示终端交互用 `<div class="term">…</div>`。内容据「内容来源」提炼，**不照搬整段 README**，按下述结构组织。页面间互链用相对路径（如 `./installation`），保证死链检查通过。

- [ ] **Step 1: what-is-ccr.md**

必含小节：
1. 一句话定位（取 README 开头 + CLAUDE.md「这是什么」）。
2. **与 cc-switch 的区别**——用一个 Markdown 表对比：维度（切换粒度 / 同时生效数 / 改全局配置吗 / 多开方式）× cc-switch vs ccr。核心：cc-switch 切全局当前 provider（同时只一个）；ccr 按终端会话注入 env，多开=多个终端各跑一次。
3. 适合谁 / 何时用。
末尾「下一步」链到 `./installation`。

- [ ] **Step 2: installation.md**

取 README「安装」四种方式，逐一给带标题代码块：
- 预编译二进制（任何系统）：到 Releases 下载、解压、放进 PATH。
- `go install`：```bash [go install]\ngo install github.com/ttsimon/cc-run/cmd/ccr@latest\n```
- Scoop（Windows）：```bash [Scoop]\nscoop bucket add ttsimon https://github.com/ttsimon/scoop-bucket\nscoop install ccr\n```
- Homebrew（macOS/Linux）：```bash [Homebrew]\nbrew install ttsimon/tap/ccr\n```
- 从源码构建：```bash [源码]\ngo build -o ccr ./cmd/ccr\n```
末尾链到 `./getting-started`，并提一句多平台构建见仓库 `RELEASING.md`（外链 GitHub）。

- [ ] **Step 3: getting-started.md**

第一次跑：
1. `ccr`（不带参数）→ 交互式选择器选一个配置启动（用终端卡演示）。
2. `ccr <名字>` 直接按名启动。
3. `ccr ls` 看所有配置（两来源）。
4. 提一句多开：开多个终端 tab、各跑一次。
用至少一个 `<div class="term">` 演示。末尾链到 `./profiles`。

- [ ] **Step 4: profiles.md**

取 CLAUDE.md「旁挂元数据层」+「按名解析」+ registry：
1. **两个来源**：cc-switch 库（只读 SQLite）+ 自定义目录 `~/.ccr/profiles/*.json`；合并、标注来源、重名用 `来源:名字` 消歧。
2. **旁挂元数据**：`~/.ccr/overlay.json`（别名+默认）、`~/.ccr/state.json`（上次用的）——为何另存（cc-switch 库只读）。
3. **按名解析顺序**：精确名 → 特殊记号 `-`（上次）/ `.`（默认）→ 别名 → 模糊子串（唯一直启、多命中弹选择器）。用表或列表讲清。
4. 相关命令：`ccr ls` / `show <名> [--reveal]` / `edit <名>` / `alias` / `unalias` / `default`（各一行说明 + 例）。
末尾链到 `./doctor`。

- [ ] **Step 5: doctor.md**

`ccr doctor [名字]`：体检后端可达性（不带名=全部）。讲用途（换机/新后端先体检）、输出怎么读、退出码语义。用终端卡演示一次。

- [ ] **Step 6: 提交**

```bash
git add website/guide/
git commit -m "docs: guide section (what-is/install/getting-started/profiles/doctor)"
```

---

## Task 5: chain 五页（chain/）

**Files:**
- Create: `website/chain/concept.md`
- Create: `website/chain/writing-a-chain.md`
- Create: `website/chain/pausing.md`
- Create: `website/chain/isolation.md`
- Create: `website/chain/security.md`

> 来源：`2026-06-09-ccr-chain-design.md` 与 `2026-06-11-ccr-chain-isolation-design.md`、`internal/chain/schema.go`。

- [ ] **Step 1: concept.md**

取 chain 设计「定位」「首条参考链」：
1. chain 是什么——多后端 agent 流水线，每段挂不同 provider。
2. 填的空白——把不同模型按各自所长串起来，官方 subagent 不好做的「异构后端串联」。
3. 首条参考链（也是内置模板）：`规划(强) → 实现(便宜) → 审查(另一家)`，设计哲学「智力前置到规划」。
4. `ccr chain init` 生成模板、`ccr chain <file>` 跑。
末尾链到 `./writing-a-chain`。

- [ ] **Step 2: writing-a-chain.md**

取 `schema.go` 字段，给**完整 yaml 示例** + 字段表：
- Chain 顶层：`name`、`isolate`、`workdir`、`segments`。
- Segment：`name`、`profile`、`prompt`（可含 `{{prev.output}}` / `{{input}}`）、`allow_tools`、`deny_commands`、`review`、`optional`。
- 模板示例（完整可跑）：
  ```yaml
  name: plan-impl-review
  isolate: true
  segments:
    - name: plan
      profile: strong
      prompt: 为「{{input}}」写详细实现计划，分阶段/任务/验证写到 docs/plans/。
    - name: impl
      profile: cheap
      prompt: 上一段产出：{{prev.output}}。照计划实现。
    - name: review
      profile: another
      prompt: 审查实现，写 findings 与判定。
      review: true
    - name: fix
      profile: cheap
      prompt: 按 findings 修复。
      optional: true
  ```
- 讲 `{{prev.output}}`（上段产出的「指路」）与 `{{input}}`（整链需求，`ccr chain <file> --input "需求"` 注入）。
末尾链到 `./pausing`。

- [ ] **Step 3: pausing.md**

取「人在环里」「审查收场」：
1. 默认**段间放行**（`⏸`：放行 / 改指令 / 跳过 / 退出），`⏸` 时可直接改工作目录里的计划/findings 再放行。
2. `--auto`：关停顿一条道跑到黑。
3. **审查判定**：审查段写 `findings` + 判定（`pass`/`needs-work`）；pass 放行即收链，needs-work 由用户决定是否进 optional 修复段；不据判定自动分叉/循环。
末尾链到 `./isolation`。

- [ ] **Step 4: isolation.md**

取 `2026-06-11-ccr-chain-isolation-design.md`（这是 track A 新语义，重点页）：
1. 为什么需要——动机事故：旧实现无条件 cleanup 销毁过成果。
2. **可插拔隔离**：git 目录用临时 worktree（每段 ccr 兜底提交）；非 git 目录用 copydir 快照。
3. **结束三态**（用表）：跑完且审查 pass → merge 合回当前分支并删临时分支；needs-work / 报错 / 退出 → 保留成果 + 打印取回路径，**绝不静默销毁**。
4. 铁律一句话框起来：`::: danger 铁律` 只有 Integrate 成功才删临时分支，其余路径都保留成果。
5. 风险提醒（worktree 只覆盖 git 跟踪文件等），用 `::: warning`。
末尾链到 `./security`。

- [ ] **Step 5: security.md**

取「安全：四层纵深」：
1. 四层：可插拔隔离（可回滚/保留）、每段 `allow_tools` 白名单、`PreToolUse` 钩子命令红线黑名单（内置默认 + yaml 追加，命中即 halt）、写操作圈在工作目录内。
2. `deny_commands` 怎么写、内置默认含哪些类（如 `rm -rf`、`git push` 等危险项）。
3. `::: warning` 提醒 Windows 真沙箱弱，主要靠隔离 + 钩子兜底。

- [ ] **Step 6: 提交**

```bash
git add website/chain/
git commit -m "docs: chain section (concept/writing/pausing/isolation/security)"
```

---

## Task 6: 参考三页（reference/）

**Files:**
- Create: `website/reference/commands.md`
- Create: `website/reference/files.md`
- Create: `website/reference/faq.md`

> 来源：`internal/cli/cli.go` 的 `printUsage`、CLAUDE.md。

- [ ] **Step 1: commands.md**

命令速查——一个 Markdown 表（命令 / 作用 / 例），覆盖 `printUsage` 全部条目：
- `ccr`（交互选择启动）、`ccr <名|别名|前缀> [claude参数]`、`ccr -`（上次）、`ccr .`（默认）。
- `ccr ls`、`ccr show <名> [--reveal]`、`ccr edit <名>`。
- `ccr alias [<别名> <目标>]`、`ccr unalias <别名>`、`ccr default [<名>]`。
- `ccr doctor [名]`。
- `ccr chain <file> [--auto] [--input "需求"]`、`ccr chain init [模板名]`。
- `ccr completion <shell>`、`ccr completion install [shell] [--uninstall]`。
表后补：透传——`ccr <名> 后面的参数原样传给 claude`。

- [ ] **Step 2: files.md**

配置文件与路径表：
- `~/.ccr/profiles/*.json` —— 自定义 profile（JSON）。
- `~/.ccr/overlay.json` —— 别名 + 默认。
- `~/.ccr/state.json` —— 上次用的。
- `~/.ccr/config.json` —— 路径/配置覆盖（解析优先级：env > config.json > 默认）。
- `~/.cc-switch/*.db` —— cc-switch 来源（**只读**）。
- chain 运行产物 `.ccr-chain/`（settings/verdict/findings，隔离时不合回）。
每条给「路径 / 作用 / 谁写」。

- [ ] **Step 3: faq.md**

常见问题 / 故障排查（每条 Q/A）：
- 装好了但 `ccr` 找不到？→ 二进制是否在 PATH。
- 看不到 cc-switch 里的配置？→ `~/.cc-switch/*.db` 是否存在、`ccr ls` 来源标注。
- 模糊命中弹选择器了？→ 多命中行为，给更精确的名或用别名。
- chain 跑完成果去哪了？→ 见 `../chain/isolation`（三态、取回路径），相对链接。
- Windows 下补全/钩子？→ 指向 `ccr completion install`。

- [ ] **Step 4: 提交**

```bash
git add website/reference/
git commit -m "docs: reference section (commands/files/faq)"
```

---

## Task 7: GitHub Pages 部署 + 整体验证

加部署 workflow，并做全站构建（死链/搜索/视觉）验证。

**Files:**
- Create: `.github/workflows/docs.yml`

- [ ] **Step 1: 全站构建（死链闸）**

Run（`website/`）：`npm run docs:build`
Expected: **成功且零死链**。VitePress 默认死链即报错；若报死链，按提示修内部链接（确保 nav/sidebar/页内相对链接都指向已存在页面），改完重跑直至通过。

- [ ] **Step 2: 写部署 workflow**

`.github/workflows/docs.yml`：
```yaml
name: docs
on:
  push:
    branches: [master]
    paths: ['website/**', '.github/workflows/docs.yml']
  workflow_dispatch:

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: pages
  cancel-in-progress: true

jobs:
  build:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: website
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: npm
          cache-dependency-path: website/package-lock.json
      - run: npm ci
      - run: npm run docs:build
      - uses: actions/configure-pages@v5
      - uses: actions/upload-pages-artifact@v3
        with:
          path: website/.vitepress/dist

  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - id: deployment
        uses: actions/deploy-pages@v4
```

- [ ] **Step 3: 本地预览人工核对（视觉验收）**

Run（`website/`）：`npm run docs:preview`，浏览器开 `http://localhost:4173/cc-run/`，核对：
- 首页 hero 名称走品牌渐变、三个按钮、四张 feature 卡、终端卡显示正常。
- 暗/明切换都不破样式；feature 卡 hover 抬升。
- 侧边栏三组（指南/chain/参考）齐全可达；本页目录（outline）出现。
- 搜索框输入中文词（如「隔离」「别名」）能搜到对应页。
- 768px 以下 hero 堆叠、布局不溢出。
Expected: 以上均符合 spec；不符则回到对应 Task 修正。

- [ ] **Step 4: 提交**

```bash
git add .github/workflows/docs.yml
git commit -m "ci: build and deploy docs to GitHub Pages on master"
```

- [ ] **Step 5: 仓库侧一次性设置（人工，非代码）**

合并到 master 后，在 GitHub 仓库 Settings → Pages → Source 选 **GitHub Actions**。首次 workflow 跑完，站点上线于 `https://ttsimon.github.io/cc-run/`。（此步无法用代码完成，记入交付说明。）

---

## Self-Review

**Spec coverage（对照 docs-site-design.md）：**
- 多页文档站 + 侧边栏 + 本地搜索 → Task 2 config（sidebar/search）。✓
- VitePress 工程、`website/` 隔离布局 → Task 1 + File Structure。✓
- 中文优先 + i18n 友好（root locale，预留 en）→ Task 2 `locales`。✓
- 暗色品牌主题（色板映射 VitePress 变量、终端卡）→ Task 2 custom.css（逐字取自 spec）。✓
- 落地首页 hero + 4 feature 卡（含确切文案）→ Task 3。✓
- 信息架构三区（guide 5 / chain 5 / reference 3）→ Task 4/5/6（页数与 spec 一致）。✓
- 隔离页承载 track A 新语义 + 铁律 callout → Task 5 Step 4。✓
- GitHub Pages 自动部署（push master 改 website/** 触发）→ Task 7 workflow。✓
- base `/cc-run/` → Task 2 config。✓
- 验证：docs:build 死链闸 + 视觉人工核对 → Task 7 Step 1/3。✓
- 贡献链到 CONTRIBUTING（不重写）→ 体现在 commands/faq 外链；首页/nav 未单列贡献项（spec 说 footer/nav 链接，可选，未做硬性页面——符合 YAGNI，不算缺口）。
- 非目标（英文内容、自定义域名、版本化、博客页）→ 计划均未做。✓

**Placeholder scan：** 内容页用「必含小节 + 来源映射 + 关键代码/yaml 片段」形式，是具体内容简报而非 "TODO 写内容"；所有 config/theme/workflow/首页为逐字完整内容。Task 2 Step 4 与 Task 3 Step 2 明确说明「死链报错属预期、Task 7 整体验证」，不是含糊其辞。无 TBD。

**一致性：** `base: '/cc-run/'` 在 config 与预览 URL、workflow 一致；sidebar 链接路径（`/guide/…`、`/chain/…`、`/reference/…`）与 Task 4/5/6 创建的文件名逐一对应；custom.css 的类名（`.term`/`.term-bar`/`.term-dot`/`.VPFeature`）与首页 HTML 一致；`vitepress ^1.6.3` 与 `setup-node 20`、`upload-pages-artifact@v3`/`deploy-pages@v4` 版本自洽。
