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
