# FAQ / 故障排查

## Q: 装好了但 `ccr` 找不到？

先确认二进制文件在 `PATH` 里：

```bash
$ ccr --version
ccr v0.1.0 (commit abc1234, built 2026-06-01)
```

如果提示 `command not found`，检查安装路径是否在 `PATH` 中。Windows 下用 Scoop 安装的，确认 Scoop shims 目录已加入 `PATH`。

## Q: 看不到 cc-switch 里的配置？

```bash
$ ccr ls
```

确认 `~/.cc-switch/cc-switch.db`（或 `CCR_DB` 指向的路径）存在且可读。如果 db 路径不在默认位置，用环境变量覆盖：

```bash
$ export CCR_DB=/path/to/cc-switch.db
$ ccr ls
```

## Q: 名字冲突了怎么办？

两个来源有同名配置时，用**限定名**消歧：

```bash
$ ccr cc-switch:DeepSeek     # 来自 cc-switch 的 DeepSeek
$ ccr custom:DeepSeek        # 来自自定义目录的 DeepSeek
```

不需要限定时直接用名字：**精确名优先**，不会因为另一个来源有同名就报错。

## Q: 模糊命中弹选择器了？

多个 profile 匹配到同一个子串时，CC Run 会弹出选择器让你挑一个。解决方法：

1. **输入更精确的名字**——少打几个字是图方便，但名字越精确越不容易冲突
2. **设别名**——把常用的简写映射到确定目标：

```bash
$ ccr alias ds deepseek-chat
$ ccr ds           # 精确命中别名，不会弹选择器
```

## Q: `ccr -` / `ccr .` 报错？

```bash
$ ccr -
还没有「上次」记录；先用 `ccr <名字>` 跑一次。

$ ccr .
还没设默认；用 `ccr default <名字>` 设置。
```

- `ccr -` 需要**至少跑过一次** `ccr <名字>`，才会有"上次"记录
- `ccr .` 需要**先设置默认配置**：`ccr default <名字>`

## Q: chain 跑完成果去哪了？

开了隔离（`isolate: true`）时，链跑完后引擎按「最后一次审查的 verdict」**自动**决定成果去留，fail-closed：

- **verdict = pass**（或链中无审查段）→ 自动合并回当前分支
- **verdict = needs-work / 判定缺失 / 出错 / 中途退出** → 成果保留、打印取回路径，绝不静默销毁

worktree 场景成果以提交留在临时分支上（`git merge <分支>` 取回）；copydir 场景临时目录整个保留给你。详见 [chain 隔离与成果交回](../chain/isolation.md)。

## Q: Windows 下补全怎么配？

最简单的方式——一键安装，自动探测当前 shell：

```bash [PowerShell]
$ ccr completion install
已把补全写入 C:\Users\...\Documents\PowerShell\Microsoft.PowerShell_profile.ps1。重开终端或 source 它生效。
```

```bash [Git Bash]
$ ccr completion install
已把补全写入 ~/.bash_profile。重开终端或 source 它生效。
```

或者手动打印脚本后自行管理：

```bash [PowerShell]
$ ccr completion powershell > $PROFILE
```

```bash [Bash]
$ ccr completion bash >> ~/.bashrc
```

```bash [Zsh]
$ ccr completion zsh >> ~/.zshrc
```

卸载也只需一行：

```bash
$ ccr completion install --uninstall
```

## Q: token 怎么隐藏？

`ccr show` 默认对 token **打码显示**（只露前 4 位 + 后 4 位）：

```bash
$ ccr show deepseek
  ANTHROPIC_AUTH_TOKEN=sk-F...abcd   # 默认打码
```

加 `--reveal` 显示完整明文：

```bash
$ ccr show deepseek --reveal
  ANTHROPIC_AUTH_TOKEN=sk-FAKE-example-full-token
```

<Badge type="warning" text="谨慎" /> 截图、录屏、分享前确认没有开着 `--reveal` 的终端。

## Q: 怎么加新后端？

用 `ccr edit` 一步到位：

```bash
$ ccr edit my-new-backend
```

首次创建会自动带入模板，填好 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN` 保存即生效。

文件保存在 `~/.ccr/profiles/my-new-backend.json`。也可以手动在该目录下创建 JSON 文件，格式见 [配置文件与路径](./files)。

## Q: CC Run 和 cc-switch 怎么配合？

两者的定位互补，不冲突：

| | cc-switch | CC Run |
|------|-----------|-----|
| 粒度 | 全局 | 当前终端 |
| 配置存储 | SQLite（`~/.cc-switch/`） | JSON（`~/.ccr/profiles/`） |
| 修改方式 | cc-switch 自己的命令 | `ccr edit` |
| CC Run 如何使用 | **只读**，自动读入 | 合并展示，同名消歧 |

**一句话**：cc-switch 管"这台机器有哪些后端"，CC Run 管"这个终端用哪个"。CC Run 自动读取 cc-switch 的配置（不会修改），同时允许你通过自定义目录添加额外后端。

## 更多信息

- 所有命令速查 → [命令速查](./commands)
- 配置文件与路径 → [配置文件与路径](./files)
- 配置来源与按名解析 → [配置与按名解析](../guide/profiles)
