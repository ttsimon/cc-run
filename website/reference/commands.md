# 命令速查

`ccr` 的命令分五组：启动、管理、元数据、工具、补全。

## 启动类

| 命令 | 作用 | 示例 |
|------|------|------|
| `ccr` | 交互式选择配置启动 | `ccr` |
| `ccr <名\|别名\|前缀>` | 按名/别名/模糊命中启动，多余参数透传给 claude | `ccr deepseek -p "你好"` |
| `ccr -` | 重跑上次用的配置 | `ccr -` |
| `ccr .` | 跑默认配置（先 `ccr default` 设过） | `ccr .` |

## 管理类

| 命令 | 作用 | 示例 |
|------|------|------|
| `ccr ls` | 列出所有配置（两来源） | `ccr ls` |
| `ccr show <名字> [--reveal]` | 查看某配置（默认 token 打码） | `ccr show deepseek --reveal` |
| `ccr edit <名字>` | 用 `$EDITOR` 编辑/新建自定义配置 | `ccr edit my-profile` |

## 元数据类

| 命令 | 作用 | 示例 |
|------|------|------|
| `ccr alias` | 列出所有别名 | `ccr alias` |
| `ccr alias <别名> <目标>` | 设置别名 | `ccr alias prod deepseek` |
| `ccr unalias <别名>` | 删除别名 | `ccr unalias prod` |
| `ccr default` | 查看当前默认 | `ccr default` |
| `ccr default <名字>` | 设置默认配置 | `ccr default deepseek` |

## 工具类

| 命令 | 作用 | 示例 |
|------|------|------|
| `ccr doctor [名]` <Badge type="info" text="v0.3" /> | 体检后端可达性 | `ccr doctor` |
| `ccr chain <file> [--auto] [--input \| -i "需求"] [-q \| -v]` <Badge type="info" text="v0.3" /> | 跑一条链（`-q` 静默 / `-v` 详细，二者互斥） | `ccr chain plan.chain.yaml --auto` |
| `ccr chain init [模板名]` <Badge type="info" text="v0.3" /> | 生成链模板到 `<模板名>.chain.yaml`（缺省模板 `plan-impl-review`） | `ccr chain init` |

## 补全类

| 命令 | 作用 | 示例 |
|------|------|------|
| `ccr completion <shell>` | 打印补全脚本（bash/zsh/powershell） | `ccr completion bash` |
| `ccr completion install [shell] [--uninstall]` | 一键装/卸补全到当前 shell 配置 | `ccr completion install` |

## 透传参数

`ccr <名>` 后面的参数会**原样透传**给 `claude`：

```bash
$ ccr deepseek -p "这段代码有什么问题？"
$ ccr kimi --model haiku
$ ccr glm --continue
```

## 消歧

当两个来源有同名配置时，用 `来源:名字` 限定：

```bash
$ ccr cc-switch:DeepSeek     # 来自 cc-switch 的 DeepSeek
$ ccr custom:DeepSeek        # 来自自定义目录的 DeepSeek
```

不需要限定名时，直接写名字即可——**精确名优先**，不会因为存在另一个来源的同名配置而报错。

## 更多信息

- 配置来源与按名解析规则 → [配置与按名解析](../guide/profiles)
- 了解 chain 多后端流水线 → [chain 概念](../chain/concept.md)
- 配置文件与路径细节 → [配置文件与路径](./files)
