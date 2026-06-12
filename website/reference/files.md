# 配置文件与路径

ccr 的配置、元数据、运行时产物分布在以下路径。

## 配置与数据文件

| 路径 | 作用 | 谁写 |
|------|------|------|
| `~/.ccr/profiles/*.json` | 自定义 profile（每个文件一个配置，文件名即配置名） | 用户（`ccr edit`） |
| `~/.ccr/overlay.json` | 别名映射 + 默认配置 | `ccr alias` / `ccr default` |
| `~/.ccr/state.json` | 上次使用的配置（供 `ccr -` 重跑） | 自动 |
| `~/.ccr/config.json` | 路径覆盖（db 位置、profiles 目录） | 用户手动 |
| `~/.cc-switch/*.db` | cc-switch 管理的 provider 数据库 | cc-switch（**只读**） |
| `~/.cc-switch/cc-switch.db` | 默认 cc-switch 数据库路径 | cc-switch（**只读**） |
| `.ccr-chain/` | chain 运行时产物（settings/verdict/findings） | `ccr chain` |

## 环境变量覆盖

路径的优先级：**环境变量 > `~/.ccr/config.json` > 默认值**。

| 环境变量 | 用途 | 默认值 |
|----------|------|--------|
| `CCR_DB` | 覆盖 cc-switch 数据库路径 | `~/.cc-switch/cc-switch.db` |
| `CCR_PROFILES_DIR` | 覆盖自定义 profiles 目录 | `~/.ccr/profiles/` |
| `EDITOR` | `ccr edit` 使用的编辑器 | `vi`（Unix）/ `notepad`（Windows） |

## config.json

路径覆盖也可以写到文件——适合想固定路径但不想每次设环境变量的场景：

```json
{
  "db": "/custom/path/cc-switch.db",
  "profilesDir": "/my-profiles"
}
```

两个键都可选，只写需要覆盖的即可。

## Profile JSON 格式

`~/.ccr/profiles/` 下的每个 `.json` 文件就是一个 profile。格式：

```json
{
  "model": "sonnet",
  "env": {
    "ANTHROPIC_BASE_URL": "https://api.example.com/anthropic",
    "ANTHROPIC_AUTH_TOKEN": "sk-FAKE-example-token"
  }
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `model` | 否 | 模型名，如 `sonnet`、`opus`、`haiku` |
| `env` | 是 | 键值对，注入为环境变量。至少需要 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN` |

用 `ccr edit <名字>` 新建时会自动带入模板，填好键值保存即可。

## overlay.json

别名和默认配置的存储格式：

```json
{
  "default": "deepseek",
  "aliases": {
    "prod": "deepseek",
    "g": "glm"
  }
}
```

## state.json

上次使用的记录，格式简洁：

```json
{
  "last": "ccswitch:deepseek"
}
```

限定名（`来源:名字`）确保 `ccr -` 重放时精确命中。

## chain 产物

`.ccr-chain/` 目录由 `ccr chain` 管理，存放链执行过程中的状态、审查结论和输出成果。具体路径和交回机制见 [chain 隔离与成果交回](../chain/isolation.md)。

## 更多信息

- 两个配置来源的合并与消歧 → [配置与按名解析](../guide/profiles)
- 所有命令的速查表 → [命令速查](./commands)
