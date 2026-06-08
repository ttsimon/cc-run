# 开发与贡献指南

## 环境准备

需要 Go（见 `go.mod` 的版本）。开发工具（按需安装）：

```bash
go install github.com/go-task/task/v3/cmd/task@latest                 # 任务入口
go install github.com/evilmartians/lefthook@latest                    # git 钩子
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
go install github.com/zricethezav/gitleaks/v8@latest                  # 密钥扫描
```

克隆后启用 git 钩子（**只需一次**）：

```bash
task hooks      # = lefthook install
```

## 常用命令（统一走 task）

```bash
task            # 列出全部任务
task build      # 构建 ccr
task test       # 跑测试
task lint       # golangci-lint
task check      # 提交前全套：fmt + vet + lint + test
task vuln       # 漏洞扫描 (govulncheck)
task secrets    # 全仓库密钥扫描 (gitleaks)
task snapshot   # 本地试打包（不发布）
```

## 提交规范（强制）

提交信息必须符合 [Conventional Commits](https://www.conventionalcommits.org/)，由 `commit-msg` 钩子（`tools/commitlint`）自动校验，不合规会被拒绝。

```
<类型>[(可选范围)][!]: <描述>
```

类型：`feat` `fix` `docs` `style` `refactor` `perf` `test` `build` `ci` `chore` `revert` `security`。

其中 `feat:` / `fix:` 会进发布说明（见 [RELEASING.md](RELEASING.md)），其余不进。**带破坏性变更**在类型后加 `!`（如 `feat!:`）。

## 提交时自动发生什么

`lefthook` 钩子：

| 时机 | 动作 |
|---|---|
| pre-commit | `gofmt` 自动格式化并重新 stage、`golangci-lint`、`gitleaks` 密钥扫描 |
| commit-msg | Conventional Commits 校验 |
| pre-push | `go test ./...` |

## 密钥安全

**绝不提交真实 token / API key**，哪怕在测试或文档里。测试用的假值请带 `FAKE` 标记（如 `sk-FAKE...`），已在 `.gitleaks.toml` 放行；真实随机密钥仍会被 gitleaks 拦截。

## 分支保护（仓库管理员一次性设置）

`master` 建议开启保护：要求 PR、要求 CI 通过再合并。用 GitHub CLI：

```bash
gh api -X PUT repos/ttsimon/cc-run/branches/master/protection \
  -F required_status_checks.strict=true \
  -F 'required_status_checks.contexts[]=test (ubuntu-latest)' \
  -F 'required_status_checks.contexts[]=golangci-lint' \
  -F 'required_status_checks.contexts[]=security' \
  -F enforce_admins=true \
  -F required_pull_request_reviews.required_approving_review_count=0 \
  -F restrictions=
```

或在网页：Settings → Branches → Add branch ruleset，勾选 *Require status checks to pass*（选 `test`、`golangci-lint`、`security`）。
