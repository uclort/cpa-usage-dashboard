package main

import "net/http"

func main() {}

func pluginRegistration() registration {
	return registration{
		SchemaVersion: schemaVersion,
		Metadata: metadata{
			Name:             "用量看板",
			Version:          pluginVersion,
			Author:           "uclort",
			GitHubRepository: "https://github.com/uclort/cpa-usage-dashboard",
			ConfigFields: []configField{
				{Name: "cache-ttl", Type: "string", Description: "看板缓存时间，默认 5m，支持 1m 到 24h。"},
				{Name: "request-timeout", Type: "string", Description: "单个用量源请求超时，默认 15s。"},
				{Name: "oauth-auth-indexes", Type: "array", Description: "加入看板的 Codex、Gemini、Antigravity OAuth 凭证索引。"},
			},
		},
		Capabilities: registrationCapabilities{ManagementAPI: true, UsagePlugin: true},
	}
}

func managementRegistration() managementRegistrationResponse {
	return managementRegistrationResponse{
		Routes: []managementRoute{
			{Method: http.MethodGet, Path: overviewRoute, Description: "返回所有自定义用量源看板数据。"},
			{Method: http.MethodGet, Path: credentialsRoute, Description: "返回可绑定的 CLIProxyAPI 凭证清单。"},
			{Method: http.MethodGet, Path: "/panel", Menu: "用量看板", Description: "自定义 HTTP 用量源看板。"},
		},
	}
}
