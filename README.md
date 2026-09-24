# CPA Usage Dashboard

CLIProxyAPI 原生通用用量看板插件。通过页面配置 HTTP 请求和 JSON 字段路径，将不同服务返回的用量数据统一展示为卡片和进度条。

## 功能

- 独立的“用量看板”菜单
- 自定义 HTTP GET / POST 请求
- 自定义查询 Token，或绑定 CLIProxyAPI 已有凭证
- 自定义请求头和请求体
- 使用点号路径解析 JSON
- 数值除法、乘法和单位换算
- 缓存及手动强制刷新
- Linux AMD64 / ARM64

## OAuth 凭证

在面板的「OAuth 凭证」页勾选已有的 Codex 或 Gemini OAuth 凭证，保存后会直接展示这些凭证的官方用量，不需要手写查询接口或解析规则。

## 通用 HTTP 用量源示例

假设服务返回：

```json
{
  "data": {
    "items": [
      {
        "label": "Standard Plan",
        "limit": 1000,
        "consumed": 250,
        "reset_at": 1800000000,
        "state": "active"
      }
    ]
  }
}
```

对应配置：

```yaml
sources:
  - id: example-service
    name: Example Service
    enabled: true
    method: GET
    url: https://api.example.com/v1/usage
    auth_mode: custom
    custom_token: YOUR_QUERY_TOKEN
    auth_header: Authorization
    auth_prefix: "Bearer "
    headers:
      Accept: application/json
      X-Tenant-ID: example
    list_path: data.items
    field_name: label
    field_total: limit
    field_used: consumed
    field_status: state
    field_reset_at: reset_at
    divisor: 1
    multiplier: 1
    unit: credits
```

绑定已有凭证时，将鉴权部分改为：

```yaml
    auth_mode: credential
    auth_index: AUTH_INDEX
    auth_field: access_token
    auth_header: Authorization
    auth_prefix: "Bearer "
```

`auth_mode: custom` 使用独立查询 Token；`auth_mode: credential` 从指定凭证字段读取 Token。两种模式互不混用。
