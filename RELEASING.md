# 发布指南

`ccr` 用 [GoReleaser](https://goreleaser.com) 一键产出全平台二进制并分发到多个安装渠道。本文分两部分：**一次性设置**（首发前做一遍）和**每次发布**。

---

## 一、一次性设置

### 1. 创建三个仓库（都在 `ttsimon` 名下）

| 仓库 | 用途 | 必须？ |
|---|---|---|
| `ttsimon/cc-run` | 主仓库（代码 + Release） | 是 |
| `ttsimon/scoop-bucket` | Scoop manifest（`scoop install ccr`） | 想要 Scoop 才需要 |
| `ttsimon/homebrew-tap` | Homebrew formula（`brew install`） | 想要 Homebrew 才需要 |

用 `gh` 创建（已登录 GitHub CLI 的话）：
```bash
gh repo create ttsimon/cc-run --public --source=. --remote=origin --push
gh repo create ttsimon/scoop-bucket --public --description "Scoop bucket for ccr"
gh repo create ttsimon/homebrew-tap --public --description "Homebrew tap for ccr"
```
> scoop-bucket / homebrew-tap 建成空仓库即可，GoReleaser 发布时会自动往里写文件。

> winget 暂未启用。日后想上架（manifest 进微软官方 `microsoft/winget-pkgs`，需 fork + PR 审核，未签名二进制有 SmartScreen 警告），按 <https://goreleaser.com/customization/winget/> 在 `.goreleaser.yaml` 补一个 `winget:` 段即可。

### 2. 配置 PAT secret

本仓库的 Release 用 Actions 内置的 `GITHUB_TOKEN` 就够；但**推送到 scoop-bucket / homebrew-tap 是跨仓库写**，内置 token 没权限，需要一个 Personal Access Token：

1. GitHub → Settings → Developer settings → **Personal access tokens**
   - Fine-grained：对 `scoop-bucket`、`homebrew-tap` 两个仓库授予 **Contents: Read and write**；
   - 或 Classic：勾选 `repo` 即可。
2. 主仓库 → Settings → Secrets and variables → Actions → **New repository secret**
   - Name：`GORELEASER_TOKEN`
   - Value：上一步的 PAT

---

## 二、每次发布

确保 `main` 干净、测试通过：
```bash
go test ./...
```

打 tag 并推送（语义化版本，前缀 `v`）：
```bash
git tag v0.1.0
git push origin v0.1.0
```

推送后：
- GitHub Actions 的 `release` 工作流自动运行 GoReleaser；
- 产出全平台压缩包 + `checksums.txt`，创建 GitHub Release；
- 更新 `scoop-bucket`、`homebrew-tap`。

### 本地预演（不发布）

发布前想先在本地看产物：
```bash
goreleaser release --snapshot --clean
```
产物在 `dist/`。校验配置是否合法：
```bash
goreleaser check
```

---

## 版本号注入

二进制内的版本来自构建时 `-ldflags`（GoReleaser 自动设置），运行 `ccr --version` 可见。本地 `go build` 不注入时显示 `dev`。
