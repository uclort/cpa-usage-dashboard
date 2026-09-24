package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	codexQuotaURL         = "https://chatgpt.com/backend-api/wham/usage"
	geminiQuotaURL        = "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota"
	antigravityQuotaURL   = "https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels"
	antigravityDailyURL   = "https://daily-cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels"
	antigravitySandboxURL = "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:fetchAvailableModels"
)

func oauthSupported(provider string) bool {
	return provider == "codex" || provider == "gemini" || provider == "gemini-cli" || provider == "antigravity"
}

func normalizedOAuthProvider(entry hostAuthFileEntry) string {
	provider := strings.ToLower(strings.TrimSpace(firstNonEmpty(entry.Provider, entry.Type)))
	if provider == "gemini" {
		return "gemini-cli"
	}
	return provider
}

func fetchOAuthUsage(ctx context.Context, host hostClient, cfg pluginConfig, entry hostAuthFileEntry) overviewSource {
	provider := normalizedOAuthProvider(entry)
	out := overviewSource{ID: entry.AuthIndex, Name: firstNonEmpty(entry.Email, entry.Name, provider), Status: "error", Items: []usageItem{}}
	raw, err := host.getAuth(ctx, entry.AuthIndex)
	if err != nil {
		out.Error = &usageItemError{Code: "auth_read_failed", Message: err.Error()}
		return out
	}
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	switch provider {
	case "codex":
		token := oauthLookupString(doc, "access_token")
		accountID := firstNonEmpty(oauthLookupString(doc, "account_id"), oauthLookupString(doc, "chatgpt_account_id"), accountIDFromJWT(oauthLookupString(doc, "id_token")))
		if token == "" || accountID == "" {
			out.Error = &usageItemError{Code: "credential_incomplete", Message: "Codex 凭证缺少 access_token 或 account_id"}
			return out
		}
		resp, err := doRequest(ctx, host, cfg, hostHTTPRequest{
			Method: http.MethodGet, URL: codexQuotaURL,
			Headers: map[string][]string{
				"Authorization":      {"Bearer " + token},
				"Chatgpt-Account-Id": {accountID},
				"Accept":             {"application/json"},
			},
		})
		if err != nil {
			out.Error = &usageItemError{Code: "request_failed", Message: err.Error()}
			return out
		}
		var payload map[string]any
		_ = json.Unmarshal(resp.Body, &payload)
		rate, _ := payload["rate_limit"].(map[string]any)
		for _, window := range []struct{ name, key string }{{"5 小时窗口", "primary_window"}, {"7 天窗口", "secondary_window"}} {
			value, _ := rate[window.key].(map[string]any)
			if value == nil {
				continue
			}
			used := oauthNumber(value["used_percent"])
			if math.IsNaN(used) {
				continue
			}
			item := usageItem{Name: "Codex · " + window.name, Unit: "%", UsedPercent: used, Remaining: math.Max(0, 100-used), ResetAt: oauthUnixTime(value["reset_at"])}
			out.Items = append(out.Items, item)
		}
	case "gemini", "gemini-cli":
		token := oauthLookupString(doc, "access_token")
		projectID := firstNonEmpty(entry.ProjectID, oauthLookupString(doc, "project_id"))
		if token == "" || projectID == "" {
			out.Error = &usageItemError{Code: "credential_incomplete", Message: "Gemini 凭证缺少 access_token 或 project_id"}
			return out
		}
		body, _ := json.Marshal(map[string]any{"project": projectID})
		resp, err := doRequest(ctx, host, cfg, hostHTTPRequest{Method: http.MethodPost, URL: geminiQuotaURL, Body: body, Headers: googleOAuthHeaders(token, "IDE_UNSPECIFIED")})
		if err != nil {
			out.Error = &usageItemError{Code: "request_failed", Message: err.Error()}
			return out
		}
		var payload struct {
			Buckets []map[string]any `json:"buckets"`
		}
		_ = json.Unmarshal(resp.Body, &payload)
		for _, bucket := range payload.Buckets {
			name := firstNonEmpty(oauthText(bucket["modelId"]), oauthText(bucket["model_id"]))
			if name == "" {
				continue
			}
			remainingRaw := firstValueAny(bucket["remainingFraction"], bucket["remaining_fraction"])
			remaining := math.Max(0, math.Min(100, oauthNumber(remainingRaw)*100))
			resetAt := oauthParseTime(firstNonEmpty(oauthText(bucket["resetTime"]), oauthText(bucket["reset_time"])))
			out.Items = append(out.Items, usageItem{Name: "Gemini · " + name, Unit: "%", Used: 100 - remaining, Remaining: remaining, UsedPercent: 100 - remaining, ResetAt: resetAt})
		}
		sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].Name < out.Items[j].Name })
	case "antigravity":
		token := oauthLookupString(doc, "access_token")
		projectID := firstNonEmpty(entry.ProjectID, oauthLookupString(doc, "project_id"))
		if token == "" || projectID == "" {
			out.Error = &usageItemError{Code: "credential_incomplete", Message: "Antigravity 凭证缺少 access_token 或 project_id"}
			return out
		}
		body, _ := json.Marshal(map[string]any{"project": projectID})
		for _, endpoint := range []string{antigravityQuotaURL, antigravityDailyURL, antigravitySandboxURL} {
			resp, err := doRequest(ctx, host, cfg, hostHTTPRequest{Method: http.MethodPost, URL: endpoint, Body: body, Headers: googleOAuthHeaders(token, "ANTIGRAVITY")})
			if err != nil {
				continue
			}
			var payload struct {
				Models map[string]map[string]any `json:"models"`
			}
			if json.Unmarshal(resp.Body, &payload) != nil {
				continue
			}
			for model, entry := range payload.Models {
				quota, _ := entry["quotaInfo"].(map[string]any)
				if quota == nil {
					quota, _ = entry["quota_info"].(map[string]any)
				}
				if quota == nil {
					continue
				}
				remainingRaw := firstValueAny(quota["remainingFraction"], quota["remaining_fraction"])
				remaining := math.Max(0, math.Min(100, oauthNumber(remainingRaw)*100))
				resetAt := oauthParseTime(firstNonEmpty(oauthText(quota["resetTime"]), oauthText(quota["reset_time"])))
				out.Items = append(out.Items, usageItem{Name: "Antigravity · " + model, Unit: "%", Used: 100 - remaining, Remaining: remaining, UsedPercent: 100 - remaining, ResetAt: resetAt})
			}
			if len(out.Items) > 0 {
				sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].Name < out.Items[j].Name })
				break
			}
		}
	default:
		out.Error = &usageItemError{Code: "unsupported_provider", Message: "仅支持 Codex、Gemini 和 Antigravity OAuth 凭证"}
		return out
	}
	if len(out.Items) == 0 {
		if out.Error == nil {
			out.Error = &usageItemError{Code: "no_usage_data", Message: "上游未返回可用用量"}
		}
		return out
	}
	out.Status = "ok"
	out.FetchedAt = time.Now().UTC()
	return out
}

func doRequest(ctx context.Context, host hostClient, cfg pluginConfig, request hostHTTPRequest) (hostHTTPResponse, error) {
	request.HostCallbackID = hostCallbackID(ctx)
	requestCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
	defer cancel()
	resp, err := host.doHTTP(requestCtx, request)
	if err != nil {
		return hostHTTPResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return hostHTTPResponse{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

func oauthLookupString(value any, key string) string {
	if object, ok := value.(map[string]any); ok {
		if text := oauthText(object[key]); text != "" {
			return text
		}
		for _, child := range object {
			if text := oauthLookupString(child, key); text != "" {
				return text
			}
		}
	}
	if array, ok := value.([]any); ok {
		for _, child := range array {
			if text := oauthLookupString(child, key); text != "" {
				return text
			}
		}
	}
	return ""
}

func oauthText(value any) string { text, _ := value.(string); return strings.TrimSpace(text) }

func accountIDFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var doc map[string]any
	if json.Unmarshal(payload, &doc) != nil {
		return ""
	}
	if value := oauthText(doc["chatgpt_account_id"]); value != "" {
		return value
	}
	if auth, ok := doc["https://api.openai.com/auth"].(map[string]any); ok {
		return oauthText(auth["chatgpt_account_id"])
	}
	return ""
}

func oauthNumber(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case string:
		var parsed float64
		_, _ = fmt.Sscanf(typed, "%g", &parsed)
		return parsed
	}
	return math.NaN()
}

func oauthUnixTime(value any) *time.Time {
	seconds := oauthNumber(value)
	if math.IsNaN(seconds) || seconds <= 0 {
		return nil
	}
	t := time.Unix(int64(seconds), 0).UTC()
	return &t
}

func oauthParseTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}

func firstValueAny(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func hostCallbackID(ctx context.Context) string {
	if value, ok := ctx.Value(bodyKey{}).(string); ok {
		return value
	}
	return ""
}

func googleOAuthHeaders(token, ideType string) map[string][]string {
	metadata, _ := json.Marshal(map[string]string{"ideType": ideType, "platform": "PLATFORM_UNSPECIFIED", "pluginType": "GEMINI"})
	return map[string][]string{
		"Authorization":     {"Bearer " + token},
		"Content-Type":      {"application/json"},
		"Accept":            {"application/json"},
		"User-Agent":        {"google-api-nodejs-client/9.15.1"},
		"X-Goog-Api-Client": {"google-cloud-sdk vscode_cloudshelleditor/0.1"},
		"Client-Metadata":   {string(metadata)},
	}
}
