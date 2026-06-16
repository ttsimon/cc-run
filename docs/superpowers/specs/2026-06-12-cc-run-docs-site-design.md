# cc-run 文档站设计（VitePress）

日期：2026-06-12 ｜ 分支：feat/docs-site ｜ 范围：给 cc-run 建一个面向用户的多页文档站 + 落地首页

## 目标 / 非目标

**目标**
- 一个**多页文档站**：侧边栏、本地搜索，首页是落地页（hero + feature 卡）。
- 用 **VitePress**（Markdown 源 + 自带主题 + 本地搜索 + 一流 i18n）。
- **中文优先**，但用 i18n 友好的结构，将来加英文**零返工**。
- 同仓库托管，**GitHub Pages** 自动部署。
- 内容从现有 `README.md` / `CLAUDE.md` / chain specs 提炼重写；旧 `web/index.html` 弃用（配色搬进新主题）。

**非目标（YAGNI）**
- 英文内容（只搭 i18n 结构，不写英文页）。
- 自定义域名（留 CNAME 口子，本期用 `*.github.io`）。
- 版本化文档（multi-version）。
- 博客 / changelog 页（发布说明继续走 GitHub Releases）。
- 不重写 `CONTRIBUTING.md` / `RELEASING.md`，文档站只链接它们。

## 架构与仓库布局

同仓库新增**顶层 `website/`** 目录作为独立 VitePress 工程，与内部的 `docs/superpowers/`（设计 spec、计划）物理隔离，避免 VitePress 把内部文档当站点页面渲染。

```
website/
  package.json                 # 仅 devDependency: vitepress（pin 版本）
  .gitignore                   # node_modules / .vitepress/dist / .vitepress/cache
  index.md                     # 首页：layout: home，hero + features
  guide/
    what-is-ccr.md             # 这是什么 + 与 cc-switch 区别
    installation.md            # 安装（4 种方式 + 源码）
    getting-started.md         # 第一次跑 ccr
    profiles.md                # 配置来源与按名解析（别名/默认/-/./模糊命中）
    doctor.md                  # 后端可达性体检
  chain/
    concept.md                 # 多后端流水线是什么、填什么空白
    writing-a-chain.md         # yaml schema：segment 字段、{{prev.output}}/{{input}}
    pausing.md                 # 段间放行 / --auto / 审查判定
    isolation.md               # 隔离与成果交回（worktree/copydir + 三态）
    security.md                # 命令黑名单钩子 + allow-tools + 四层纵深
  reference/
    commands.md                # 命令速查表（全部子命令 + 旗标）
    files.md                   # 配置文件与路径（config/overlay/state）
    faq.md                     # 常见问题 / 故障排查
  .vitepress/
    config.ts                  # 站点配置（见下）
    theme/
      index.ts                 # extends 默认主题 + 引入自定义 CSS
      custom.css               # 复用旧页配色（--brand #7c9cff / --brand-2 #56e1c4，暗色）
```

`README.md` 仍是 GitHub 仓库首屏的快速入口（保留、不动）；文档站是其展开版。两者不强求逐字一致，但安装/定位口径要一致。

## 站点配置（.vitepress/config.ts 要点）

- `base: '/cc-run/'`（GitHub 项目页路径）。
- `lang` / `title` / `description`：中文，标题 `ccr`，描述沿用 README 定位句。
- **i18n**：`locales` 配 `root`（简体中文，`lang: 'zh-CN'`，挂在 `/`，URL 无前缀）。预留注释位将来加 `en`（挂 `/en/`，中文页不迁移）。
- **搜索**：`themeConfig.search = { provider: 'local' }`，开启内置 minisearch；中文用 `options.miniSearch` 放宽分词（按需配 `tokenize`/`processTerm`，保证中文可搜）。
- **导航 nav**：指南 / chain / 参考 / GitHub 链接。
- **侧边栏 sidebar**：按 `guide/`、`chain/`、`reference/` 三组，顺序同上面布局。
- **socialLinks**：GitHub `ttsimon/cc-run`。
- **editLink**（可选）：指向 GitHub `website/` 对应路径，方便改文档。
- `lastUpdated: true`。

## 信息架构（侧边栏三大区）

- **首页 `/`**：一句话定位 + 「安装」「看文档」两个按钮 + GitHub；feature 卡四张——① 同时多开各用不同后端互不干扰 ② 不改全局配置（按终端会话注入 env）③ chain 多后端流水线 ④ 跨平台单二进制。
- **指南 `/guide/`**：what-is-ccr → installation → getting-started → profiles → doctor。
- **chain `/chain/`**：concept → writing-a-chain → pausing → isolation → security。
- **参考 `/reference/`**：commands → files → faq。
- 贡献：nav/footer 链到仓库 `CONTRIBUTING.md`，不在站内重写。

## 视觉与样式设计

本节是给实现者（含其他模型）的**像素级依据**，照此落地即可，无需再设计。整体延续旧 `web/index.html` 的暗色开发者审美，但收敛进 VitePress 的设计系统（覆盖其 CSS 变量），保证主题升级不破样式。

### 设计基调

- **暗色优先**的开发者气质：近黑蓝底、冷青蓝渐变品牌色、克制的暖橙点缀、大圆角、留白充足、代码用等宽。
- 同时支持 VitePress 的明/暗切换：**暗色是主打**（按下面 token 精调），明色沿用 VitePress 默认中性灰、仅替换品牌色，保证对比度达标即可，不追求与暗色逐项对称。
- 动效极克制：仅 hover 抬升 / 颜色过渡（≈0.15s）；尊重 `prefers-reduced-motion`。

### 设计 token（映射到 VitePress CSS 变量）

落到 `website/.vitepress/theme/custom.css`。色板取自旧页：

```css
/* ── 暗色（主打）── */
.dark {
  /* 背景层次：底 < soft < 卡片 */
  --vp-c-bg:        #0c0f17;
  --vp-c-bg-alt:    #0a0d14;   /* 侧边栏 / 页脚 */
  --vp-c-bg-soft:   #121826;   /* 代码块 / 输入 */
  --vp-c-bg-elv:    #161d2e;   /* 卡片 / 悬浮 */

  /* 文字 */
  --vp-c-text-1:    #e6ebf5;   /* 正文标题 */
  --vp-c-text-2:    #94a3b8;   /* 次要 / lead */
  --vp-c-text-3:    #64748b;   /* 占位 / 弱化 */

  /* 分隔 / 边框 */
  --vp-c-divider:   #243049;
  --vp-c-border:    #243049;
  --vp-c-gutter:    #0a0d14;

  /* 品牌（链接 / 选中 / 按钮）。brand-1 偏亮做文字，brand-3 做实心底 */
  --vp-c-brand-1:   #8ea8ff;   /* 链接 / 侧栏选中文字（暗底要够亮） */
  --vp-c-brand-2:   #7c9cff;
  --vp-c-brand-3:   #5b7cf0;   /* 实心按钮底色 */
  --vp-c-brand-soft: rgba(124,156,255,0.14);

  /* 强调（badge / 自定义点缀，少量用） */
  --ccr-accent:     #ffb86b;
  --ccr-grad:       linear-gradient(120deg, #7c9cff 0%, #56e1c4 100%);
}

/* ── 明色（仅换品牌色，其余用 VitePress 默认）── */
:root {
  --vp-c-brand-1:   #4c6ef0;
  --vp-c-brand-2:   #5b7cf0;
  --vp-c-brand-3:   #7c9cff;
  --vp-c-brand-soft: rgba(92,124,240,0.12);
  --ccr-accent:     #d97706;
  --ccr-grad:       linear-gradient(120deg, #5b7cf0 0%, #14b8a6 100%);
}

/* ── 排版 ── */
:root {
  --vp-font-family-base: 'Inter','Segoe UI',system-ui,-apple-system,'Microsoft YaHei',sans-serif;
  --vp-font-family-mono: 'JetBrains Mono','Cascadia Code',Consolas,monospace;

  /* 首页 hero 名称走品牌渐变 */
  --vp-home-hero-name-color: transparent;
  --vp-home-hero-name-background: var(--ccr-grad);

  /* 按钮品牌色 */
  --vp-button-brand-bg: var(--vp-c-brand-3);
  --vp-button-brand-hover-bg: var(--vp-c-brand-2);
  --vp-button-brand-active-bg: var(--vp-c-brand-3);
  --vp-button-brand-text: #ffffff;
}
```

> 字体走系统栈即可，不强制自托管 Inter/JetBrains Mono；若 CDN 可用可在 `theme/index.ts` 引入，缺失时回退系统字体，不阻塞。

### 排版规格

- 正文 `line-height: 1.7`，最大正文宽沿用 VitePress（约 688px 内容列）。
- 标题：h1 ≈ 2.2rem / 800；h2 ≈ 1.6rem / 700（带上分隔线，VitePress 默认）；h3 ≈ 1.25rem / 600。中文字重靠系统字体，不额外加粗到失真。
- 行内 `code`：`--vp-c-bg-soft` 底、`--vp-c-brand-1` 字、小圆角。
- 链接：`--vp-c-brand-1`，hover 下划线。

### 间距 / 形状

- 圆角统一偏大：卡片 / 终端卡 / 按钮 14px，行内小元素 6–8px。
- 区块竖向呼吸：首页各 section ≥ 64px 上下留白。

### 组件规格

1. **顶部导航**：VitePress 默认 sticky + 毛玻璃。左侧 wordmark `ccr`（700 字重）后跟极小号灰色副标 `cc-run`。右侧 nav（指南/chain/参考）+ GitHub 图标 + 搜索 + 明暗切换。

2. **首页 Hero**（VitePress `layout: home` frontmatter）：
   - `name: ccr`（走品牌渐变）；`text: 给每个终端注入不同后端，再拉起 claude`；`tagline: 同时多开、各用不同后端、互不干扰，且不改全局配置。`
   - `actions`：主按钮「快速上手」(theme=brand) → `/guide/getting-started`；次按钮「为什么是 ccr」(theme=alt) → `/guide/what-is-ccr`；第三个「GitHub」(theme=alt) 外链。
   - Hero 右侧用**终端卡**作主视觉（见组件 5），通过主题 `home-hero-image` 插槽或紧随 hero 的一段 HTML 实现。

3. **Feature 卡**（首页 `features`，4 张）：每张 `icon`（emoji，沿用 CLI 气质）+ `title` + `details`：
   - 🪟 **多开互不干扰** —— 每个终端各跑一次 `ccr <名>`，各用各的后端。
   - 🔌 **不改全局配置** —— 按终端会话注入 env，cc-switch 切的是全局，ccr 切的是这个 tab。
   - ⛓️ **chain 多后端流水线** —— 规划用强模型、实现用便宜模型、审查换一家，串起来跑。
   - 📦 **跨平台单二进制** —— Win/macOS/Linux，Scoop/Homebrew/go install 都行。
   - 卡片样式覆盖：底 `--vp-c-bg-soft`、边 `--vp-c-border`、圆角 14px、hover `translateY(-2px)` + 边框转品牌色。

4. **代码块**：等宽字体、`--vp-c-bg-soft` 底，保留 VitePress 的语言标签、标题（` ```bash [安装] `）、复制按钮、行高亮。命令示例尽量带 `[标题]`。

5. **终端卡（复用动机，`.term`）**：模拟终端窗口展示 `ccr` 实操。结构：顶栏三个红/黄/绿圆点 + 卡体等宽内容。CSS 落在 `custom.css`：
   ```css
   .term { background: var(--vp-c-bg-soft); border:1px solid var(--vp-c-border);
           border-radius:14px; overflow:hidden; }
   .term-bar { display:flex; gap:6px; padding:12px 16px;
               background:rgba(0,0,0,.2); border-bottom:1px solid var(--vp-c-border); }
   .term-dot { width:12px; height:12px; border-radius:50%; }
   .term pre { margin:0; padding:18px 20px; font-family:var(--vp-font-family-mono);
               font-size:.85rem; line-height:1.7; overflow-x:auto; }
   ```
   在 markdown 里用原生 HTML 块插入；作为首页 hero 主视觉与「快速上手」页的演示。

6. **Callout / 自定义容器**：用 VitePress `::: tip / warning / danger`，左边框色重映射到品牌青蓝（tip）、暖橙 `--ccr-accent`（warning）、红（danger）。chain「隔离」「安全」页多用 warning/danger 强调铁律与红线。

7. **侧边栏**：选中项 `--vp-c-brand-1` 字色 + 左侧品牌色指示；分组标题大写小灰字；三组（指南/chain/参考）。

8. **徽章 / 表格**：命令参考页的旗标/默认值用 VitePress `<Badge>`（info/tip/warning）；表格用默认描边，表头加 `--vp-c-bg-soft` 底。

### 响应式

- 断点 768px：hero 由左右两栏堆叠为单列（文字在上、终端卡在下）；feature 卡 4→2→1 列由 VitePress 默认网格处理。
- 终端卡与代码块横向溢出可滚（`overflow-x:auto`），不撑破布局。

### 无障碍

- 文字/背景对比 ≥ WCAG AA（暗底品牌字用偏亮的 brand-1，已为此选 `#8ea8ff`）。
- 保留键盘焦点环（不 `outline:none`）；emoji 图标配文字标题，不单独承载语义。

## 构建与部署

- **GitHub Pages**，项目页 `https://ttsimon.github.io/cc-run/`，Pages 源设为「GitHub Actions」。
- 新增 `.github/workflows/docs.yml`：
  - 触发：push 到 `master` 且改动 `website/**`（或手动 `workflow_dispatch`）。
  - 步骤：checkout → setup-node（pin LTS）→ `npm ci`（在 `website/`）→ `npm run docs:build` → 上传 `website/.vitepress/dist` 为 Pages artifact → `actions/deploy-pages`。
  - 权限：`pages: write`、`id-token: write`，并发组防叠跑。
- npm scripts（`website/package.json`）：`docs:dev` / `docs:build` / `docs:preview`。

## 验证

- 本地 `npm run docs:dev` 预览；`npm run docs:build` 必须通过——VitePress **默认死链即报错**，天然校验所有内部链接，作为主要正确性闸。
- CI：在 PR 上也跑 `docs:build`（可与部署 workflow 共用一个 build job，仅部署步骤限 master），构建绿才允许合并。
- 首次部署后人工过一遍：首页 hero、四张 feature 卡、侧边栏三区可达、搜索能搜到中文词、移动端布局。

## 风险 / 取舍

- **base 路径**：项目页必须 `base: '/cc-run/'`，否则 CSS/链接 404。若以后上自定义域名（根路径），改 `base: '/'` + 加 `CNAME`。
- **中文搜索**：minisearch 默认英文分词，中文需配 `miniSearch` 选项；本期目标是「能搜到」，不追求高级分词质量。
- **Node 工具链**：VitePress 需 Node，开发者机器用 mise 管理无痛；CI 用 setup-node 固定版本，避免漂移。
- **内容漂移**：文档与代码可能不同步。缓解——文档站与代码同仓库、同 PR 改；命令参考从实际 `ccr` 帮助/子命令核对一次再落笔。
- **i18n 预留**：现在只配 `root` locale；加英文时按 VitePress 文档加 `en` locale 与 `/en/` 目录，中文页保持原路径不动。
