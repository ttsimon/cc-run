# doctor 体检

`ccr doctor` 检查后端连接是否可达——适用于换了新机器、新装 cc-switch、或新加了 provider 之后快速验证。

## 用法

```bash
$ ccr doctor          # 检查全部配置
$ ccr doctor deepseek # 只检查一个
```

<div class="term">
<div class="term-bar">
<span class="term-dot red"></span><span class="term-dot yellow"></span><span class="term-dot green"></span>
</div>
<div class="term-body">
$ ccr doctor
✓ deepseek (ccswitch)    → https://api.deepseek.com/anthropic      200 OK
✓ kimi     (ccswitch)    → https://api.moonshot.cn/anthropic        200 OK
✗ my-old   (custom)      → https://my-old.example.com/anthropic     连接超时
──────────────────────────────────────────────────
3 个配置，2 个正常，1 个异常
</div>
</div>

## 输出怎么看

- `✓` —— 后端连接正常，返回了预期响应
- `✗` —— 连接失败，可能是 URL 不对、网络不通、或 token 失效
- 每个配置会显示来源标记和实际请求的 URL

## 退出码

| 退出码 | 含义 |
|--------|------|
| 0 | 全部正常 |
| 非 0 | 至少一个异常 |

退出码与脚本结合很方便：`ccr doctor || echo "有后端连不上"`。

## 什么时候用

- 换到新机器后，确认所有后端都能连
- 新增自定义配置文件后，验证 URL 和 token 是否正确
- 排查某个 provider 故障时，单独检查它

## 下一步

→ [chain 多后端流水线](../chain/concept)
