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
<pre>$ ccr doctor
✓ deepseek             HTTP 200
✓ kimi                 HTTP 200
✗ my-old               Get "https://my-old.example.com/anthropic": dial tcp: i/o timeout</pre>
</div>

每行格式是 `<标记> <名字> <详情>`，详情是 `HTTP <状态码>` 或失败原因。

## 输出怎么看

- `✓` —— 对配置的 `ANTHROPIC_BASE_URL` 发了个 GET，**状态码 < 500** 就算后端在
- 这意味着 `401` / `403`（端点活着、只是没带凭证或鉴权失败）**也算通过**——doctor 验的是「连得上」，不是「token 对不对」
- `✗` —— 请求发不出去（URL 不对、网络不通、DNS 解析失败等），详情里是底层错误；配置缺 `ANTHROPIC_BASE_URL` 时详情是「无 ANTHROPIC_BASE_URL」

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
