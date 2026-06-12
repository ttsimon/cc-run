# 安装

任选一种安装方式。所有方式输出的二进制都是纯静态编译（`CGO_ENABLED=0`），无运行时依赖。

## 预编译二进制

到 [Releases](https://github.com/ttsimon/cc-run/releases) 下载对应系统的压缩包，解压后把 `ccr`（Windows 上是 `ccr.exe`）放入 `PATH` 中的某个目录。

```bash [下载预编译二进制]
# macOS ARM64
curl -L https://github.com/ttsimon/cc-run/releases/latest/download/ccr_darwin_arm64.tar.gz | tar xz
sudo mv ccr /usr/local/bin/

# Windows（手动下载）
# 下载 ccr_windows_amd64.zip → 解压 → 把 ccr.exe 放到 PATH 目录
```

## go install

需要有 Go 环境（1.21+）。

```bash [go install]
go install github.com/ttsimon/cc-run/cmd/ccr@latest
```

产物在 `$GOPATH/bin/ccr`，确保该目录在 PATH 中即可。

## Scoop（Windows）

```bash [Scoop]
scoop bucket add ttsimon https://github.com/ttsimon/scoop-bucket
scoop install ccr
```

通过 Scoop 安装会自动注册 shell 补全，重开终端即生效。

## Homebrew（macOS / Linux）

```bash [Homebrew]
brew install ttsimon/tap/ccr
```

通过 Homebrew 安装也会自动注册 shell 补全。

## 从源码构建

```bash [从源码构建]
git clone https://github.com/ttsimon/cc-run.git
cd cc-run
go build -o ccr ./cmd/ccr
# Windows 上产物名为 ccr.exe
```

把生成的二进制放进 `PATH`。多平台构建细节见 [RELEASING.md](https://github.com/ttsimon/cc-run/blob/master/RELEASING.md)。

## 验证安装

```bash
$ ccr --version
ccr version 0.1.2
```

如果显示版本号，安装就完成了。

## 下一步

→ [快速上手](./getting-started)
