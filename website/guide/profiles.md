# 配置与按名解析

CC Run 的配置来自两个来源，合并为一个列表。

## 两个配置来源

### 1. cc-switch 数据库（只读）

`~/.cc-switch/cc-switch.db` —— cc-switch 管理的 SQLite 库。CC Run **只读**，不写不改。其中 `app_type=claude` 的所有 provider 会被自动读入。

### 2. 自定义目录

`~/.ccr/profiles/*.json` —— 每个文件一个配置，文件名即配置名。内容格式：

```json
{
  "model": "sonnet",
  "env": {
    "ANTHROPIC_BASE_URL": "https://api.example.com/anthropic",
    "ANTHROPIC_AUTH_TOKEN": "sk-FAKE-example"
  }
}
```

`model` 可选，`env` 中的键值对应注入到环境变量。

两个来源合并后，每个配置标注来源标记。重名时用 `来源:名字` 消歧，如 `custom:DeepSeek` 或 `cc-switch:DeepSeek`。

## 旁挂元数据

cc-switch 库只读，所以别名、默认、上次用等信息无法写回 cc-switch 库。这些数据存为独立文件：

| 文件 | 内容 |
|------|------|
| `~/.ccr/overlay.json` | 别名映射 + 默认配置 |
| `~/.ccr/state.json` | 上次使用的配置名 |

## 按名解析顺序

`ccr <参数>` 按以下顺序查找：

1. **精确名 / 限定名** —— `deepseek` 或 `cc-switch:DeepSeek`
2. **特殊记号** —— `-`（上次用的）、`.`（默认）
3. **别名** —— 由 `ccr alias` 设定
4. **模糊子串** —— 命中唯一 → 直接启动；命中多个 → 弹出选择器；未命中 → 报错

## 相关命令

### ccr ls —— 列出全部配置

```bash
$ ccr ls
```

列出所有配置，显示名称、来源、别名、默认标记。

### ccr show —— 查看配置详情

```bash
$ ccr show deepseek           # token 打码显示
$ ccr show deepseek --reveal  # 完整显示（含明文 token）
```

### ccr edit —— 编辑 / 新建自定义配置

```bash
$ ccr edit my-local
```

用 `$EDITOR` 打开 `~/.ccr/profiles/my-local.json`。文件存在则编辑，不存在则创建（自动带入模板）。

### ccr alias —— 设置 / 列出别名

```bash
$ ccr alias                   # 列出所有别名
$ ccr alias prod deepseek     # 之后 ccr prod 直启
```

### ccr unalias —— 删除别名

```bash
$ ccr unalias prod
```

### ccr default —— 设置 / 查看默认

```bash
$ ccr default                 # 查看当前默认
$ ccr default deepseek        # 设为默认，之后 ccr . 直启
```

## 路径覆盖

优先级：**环境变量 > `~/.ccr/config.json` > 默认值**。

| 环境变量 | 用途 |
|----------|------|
| `CCR_DB` | cc-switch 数据库路径 |
| `CCR_PROFILES_DIR` | 自定义 profiles 目录 |

或者写到 `~/.ccr/config.json`：

```json
{
  "db": "/custom/path/cc-switch.db",
  "profilesDir": "/custom/profiles"
}
```

## 下一步

→ [doctor 体检](./doctor)
