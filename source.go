package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func fetchSource(ctx context.Context, host hostClient, cfg pluginConfig, source usageSource) overviewSource {
	out := overviewSource{ID: source.ID, Name: firstNonEmpty(source.Name, source.ID), Status: "error", Items: []usageItem{}}
	if source.ID == "" || source.URL == "" || source.FieldTotal == "" || source.FieldUsed == "" {
		out.Error = &usageItemError{Code: "invalid_source", Message: "id、url、field_total、field_used 都是必填"}
		return out
	}
	method := strings.ToUpper(firstNonEmpty(source.Method, http.MethodGet))
	if method != http.MethodGet && method != http.MethodPost {
		out.Error = &usageItemError{Code: "invalid_method", Message: "method 只支持 GET 或 POST"}
		return out
	}
	headers := map[string][]string{}
	for key, value := range source.Headers {
		headers[http.CanonicalHeaderKey(key)] = []string{value}
	}
	mode := strings.ToLower(strings.TrimSpace(source.AuthMode))
	if mode == "credential" {
		if source.AuthIndex == "" {
			out.Error = &usageItemError{Code: "auth_index_missing", Message: "请选择要绑定的凭证"}
			return out
		}
		raw, err := host.getAuth(ctx, source.AuthIndex)
		if err != nil {
			out.Error = &usageItemError{Code: "auth_read_failed", Message: err.Error()}
			return out
		}
		var doc map[string]any
		_ = json.Unmarshal(raw, &doc)
		token := lookupPath(doc, firstNonEmpty(source.AuthField, "access_token"))
		if token == "" {
			out.Error = &usageItemError{Code: "auth_field_missing", Message: "凭证里找不到 " + firstNonEmpty(source.AuthField, "access_token")}
			return out
		}
		setTokenHeader(headers, source, token)
	} else if mode == "custom" {
		if source.CustomToken == "" {
			out.Error = &usageItemError{Code: "custom_token_missing", Message: "请填写自定义查询 Token"}
			return out
		}
		setTokenHeader(headers, source, source.CustomToken)
	}
	request := hostHTTPRequest{Method: method, URL: source.URL, Headers: headers}
	if method == http.MethodPost && strings.TrimSpace(source.Body) != "" {
		request.Body = []byte(source.Body)
		if headers["Content-Type"] == nil {
			headers["Content-Type"] = []string{"application/json"}
		}
	}
	requestCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
	defer cancel()
	resp, err := host.doHTTP(requestCtx, request)
	if err != nil {
		out.Error = &usageItemError{Code: "request_failed", Message: err.Error()}
		return out
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		out.Error = &usageItemError{Code: "upstream_status", Message: fmt.Sprintf("HTTP %d", resp.StatusCode), UpstreamStatus: resp.StatusCode}
		return out
	}
	var payload any
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		out.Error = &usageItemError{Code: "invalid_json", Message: "上游返回的不是 JSON"}
		return out
	}
	list := []any{payload}
	if strings.TrimSpace(source.ListPath) != "" {
		raw := lookupPathAny(payload, source.ListPath)
		if array, ok := raw.([]any); ok {
			list = array
		}
	}
	for _, raw := range list {
		item, ok := map[string]any{}, true
		if m, valid := raw.(map[string]any); valid {
			item = m
		} else {
			ok = false
		}
		if !ok {
			continue
		}
		total := transform(numberValue(lookupPathAny(item, source.FieldTotal)), source)
		used := transform(numberValue(lookupPathAny(item, source.FieldUsed)), source)
		if math.IsNaN(total) || math.IsNaN(used) {
			continue
		}
		name := source.Name
		if source.FieldName != "" {
			name = firstNonEmpty(stringValue(lookupPathAny(item, source.FieldName)), source.Name)
		}
		usage := usageItem{
			Name:        name,
			Unit:        source.Unit,
			Total:       total,
			Used:        used,
			Remaining:   math.Max(0, total-used),
			UsedPercent: 0,
		}
		if total > 0 {
			usage.UsedPercent = math.Max(0, math.Min(100, used/total*100))
		}
		if source.FieldResetAt != "" {
			if seconds := numberValue(lookupPathAny(item, source.FieldResetAt)); !math.IsNaN(seconds) {
				t := time.Unix(int64(seconds), 0).UTC()
				usage.ResetAt = &t
			}
		}
		if source.FieldStatus != "" {
			status := strings.ToLower(stringValue(lookupPathAny(item, source.FieldStatus)))
			if status != "" && status != "active" && status != "ok" {
				continue
			}
		}
		out.Items = append(out.Items, usage)
	}
	if len(out.Items) == 0 {
		out.Error = &usageItemError{Code: "no_items", Message: "没有解析到可用用量"}
		return out
	}
	out.Status = "ok"
	out.FetchedAt = time.Now().UTC()
	return out
}

func setTokenHeader(headers map[string][]string, source usageSource, token string) {
	header := http.CanonicalHeaderKey(firstNonEmpty(source.AuthHeader, "Authorization"))
	prefix := source.AuthPrefix
	if prefix == "" {
		prefix = "Bearer "
	}
	headers[header] = []string{prefix + token}
}

func transform(value float64, source usageSource) float64 {
	if source.Divisor != 0 {
		value /= source.Divisor
	}
	if source.Multiplier != 0 {
		value *= source.Multiplier
	}
	return value
}

func numberValue(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case string:
		x, _ := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return x
	}
	return math.NaN()
}

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func lookupPath(doc map[string]any, path string) string {
	return stringValue(lookupPathAny(doc, path))
}

func lookupPathAny(value any, path string) any {
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		if part == "" {
			continue
		}
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = object[part]
	}
	return value
}
