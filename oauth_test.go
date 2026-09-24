package main

import (
	"context"
	"encoding/json"
	"testing"
)

func TestFetchOAuthUsageCodex(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"rate_limit": map[string]any{"primary_window": map[string]any{"used_percent": 30.0, "reset_at": 1800000000}}})
	host := &fakeHost{response: hostHTTPResponse{StatusCode: 200, Body: body}, auth: json.RawMessage(`{"access_token":"token","account_id":"account"}`)}
	out := fetchOAuthUsage(context.Background(), host, defaultConfig(), hostAuthFileEntry{AuthIndex: "auth", Provider: "codex", Name: "Codex", Email: "user@example.com"})
	if out.Status != "ok" || len(out.Items) != 1 || out.Items[0].UsedPercent != 30 {
		t.Fatalf("out=%#v", out)
	}
}

func TestFetchOAuthUsageGemini(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"buckets": []any{map[string]any{"modelId": "model", "remainingFraction": 0.75, "resetTime": "2026-01-01T00:00:00Z"}}})
	host := &fakeHost{response: hostHTTPResponse{StatusCode: 200, Body: body}, auth: json.RawMessage(`{"access_token":"token","project_id":"project"}`)}
	out := fetchOAuthUsage(context.Background(), host, defaultConfig(), hostAuthFileEntry{AuthIndex: "auth", Provider: "gemini", Name: "Gemini", ProjectID: "project"})
	if out.Status != "ok" || len(out.Items) != 1 || out.Items[0].Remaining != 75 {
		t.Fatalf("out=%#v", out)
	}
}

func TestAntigravityPools(t *testing.T) {
	payload := map[string]map[string]any{
		"claude-sonnet": {"quotaInfo": map[string]any{"remainingFraction": .8, "resetTime": "2026-01-01T01:00:00Z"}},
		"chat-other":    {"quotaInfo": map[string]any{"remainingFraction": .1, "resetTime": "2026-01-01T00:00:00Z"}},
	}
	items := antigravityPools(payload)
	if len(items) != 1 || items[0].Name != "Claude" {
		t.Fatalf("items=%#v", items)
	}
}

func TestAntigravitySummaryItems(t *testing.T) {
	groups := []antigravityGroup{
		{DisplayName: "Gemini Models", Buckets: []antigravityBucket{{Window: "weekly", RemainingFraction: .8, ResetTime: "2026-01-07T00:00:00Z"}, {Window: "5h", RemainingFraction: 1, ResetTime: "2026-01-01T05:00:00Z"}}},
		{DisplayName: "Claude and GPT models", Buckets: []antigravityBucket{{Window: "weekly", RemainingFraction: .7, ResetTime: "2026-01-07T00:00:00Z"}, {Window: "5h", RemainingFraction: 1, ResetTime: "2026-01-01T05:00:00Z"}}},
	}
	items := antigravitySummaryItems(groups)
	if len(items) != 4 || items[0].Name != "Gemini 周额度" || items[1].Name != "Gemini 5h" || items[2].Name != "Claude 周额度" || items[3].Name != "Claude 5h" {
		t.Fatalf("items=%#v", items)
	}
}
