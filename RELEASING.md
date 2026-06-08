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

### 2. winget（可选，上架门槛较高）

winget 的 manifest 要进微软官方仓库 `microsoft/winget-pkgs`，流程：
1. 在 GitHub 上 **fork** `microsoft/winget-pkgs` 到 `ttsimon/winget-pkgs`。
2. 发布时 GoReleaser 会往你的 fork 推一个分支，并向 `microsoft/winget-pkgs` 提 PR。
3. 微软的机器人/审核员校验通过后合并，`winget install ttsimon.ccr` 才可用（首次可能要几天）。
4. 未签名的二进制安装时用户会看到 SmartScreen 警告——要消除需代码签名证书（可后续再做）。

不想现在弄 winget，就把 `.goreleaser.yaml` 里的 `winget:` 整段删掉或注释掉，其余渠道不受影响。

### 3. 配置 PAT secret

本仓库的 Release 用 Actions 内置的 `GITHUB_TOKEN` 就够；但**推送到 scoop-bucket / homebrew-tap / winget fork 是跨仓库写**，内置 token 没权限，需要一个 Personal Access Token：

1. GitHub → Settings → Developer settings → **Personal access tokens**
   - Fine-grained：对 `scoop-bucket`、`homebrew-tap`、`winget-pkgs` 三个仓库授予 **Contents: Read and write** + **Pull requests: Read and write**；
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
- 更新 `scoop-bucket`、`homebrew-tap`；若启用了 winget，则向 `microsoft/winget-pkgs` 提 PR。

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
