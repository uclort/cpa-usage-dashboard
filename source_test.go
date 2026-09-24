package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
)

func TestTransformAndLookup(t *testing.T) {
	var payload map[string]any
	_ = json.Unmarshal([]byte(`{"data":{"items":[{"limit":1000,"consumed":250}]}}`), &payload)
	list := lookupPathAny(payload, "data.items")
	if _, ok := list.([]any); !ok {
		t.Fatalf("list=%#v", list)
	}
	item := list.([]any)[0].(map[string]any)
	got := transform(numberValue(item["limit"]), usageSource{Divisor: 10, Multiplier: 2})
	if got != 200 {
		t.Fatalf("got=%v", got)
	}
}

func TestFetchSourceParsesGenericUsage(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"data": map[string]any{"items": []any{map[string]any{"label": "Standard Plan", "limit": 1000, "consumed": 250, "reset_at": 1800000000, "state": "active"}}}})
	host := &fakeHost{response: hostHTTPResponse{StatusCode: 200, Body: body}}
	source := usageSource{ID: "example", Name: "Example Service", Enabled: true, URL: "https://api.example.com/v1/usage", ListPath: "data.items", FieldTotal: "limit", FieldUsed: "consumed", FieldName: "label", FieldStatus: "state", FieldResetAt: "reset_at", Divisor: 1, Multiplier: 1}
	out := fetchSource(context.Background(), host, defaultConfig(), source)
	if out.Status != "ok" || len(out.Items) != 1 || out.Items[0].Used != 250 || out.Items[0].Total != 1000 || out.Items[0].UsedPercent != 25 {
		t.Fatalf("out=%#v", out)
	}
}

func TestCustomTokenIsIndependentFromCredential(t *testing.T) {
	body := []byte(`{"total":100,"used":20}`)
	host := &fakeHost{response: hostHTTPResponse{StatusCode: 200, Body: body}}
	source := usageSource{ID: "custom", Name: "自定义", Enabled: true, URL: "https://example.com", AuthMode: "custom", CustomToken: "query-token", AuthHeader: "Authorization", AuthPrefix: "Bearer ", FieldTotal: "total", FieldUsed: "used"}
	out := fetchSource(context.Background(), host, defaultConfig(), source)
	if out.Status != "ok" {
		t.Fatalf("out=%#v", out)
	}
	if got := host.request.Headers["Authorization"]; len(got) != 1 || got[0] != "Bearer query-token" {
		t.Fatalf("authorization=%#v", got)
	}
}

func TestDecodeNestedYAMLConfig(t *testing.T) {
	yamlText := `cache-ttl: 5m
request-timeout: 15s
sources:
  - id: example-service
    name: Example Service
    enabled: true
    method: GET
    url: https://api.example.com/v1/usage
    auth_mode: custom
    custom_token: query-token
    headers:
      X-Tenant-ID: "example"
    list_path: data.items
    field_total: limit
    field_used: consumed
`
	raw, _ := json.Marshal(lifecycleRequest{ConfigYAML: json.RawMessage(strconv.Quote(yamlText))})
	cfg, err := decodeLifecycleConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sources) != 1 || cfg.Sources[0].CustomToken != "query-token" || cfg.Sources[0].Headers["X-Tenant-ID"] != "example" {
		t.Fatalf("cfg=%#v", cfg)
	}
}

func TestHostHTTPWireShape(t *testing.T) {
	raw := base64.StdEncoding.EncodeToString([]byte(`{"ok":true}`))
	var response hostHTTPResponse
	if err := json.Unmarshal([]byte(`{"StatusCode":200,"Body":"`+raw+`"}`), &response); err != nil {
		t.Fatal(err)
	}
	if string(response.Body) != `{"ok":true}` {
		t.Fatalf("body=%q", response.Body)
	}
}

type fakeHost struct {
	response hostHTTPResponse
	request  hostHTTPRequest
	auth     json.RawMessage
}

func (f *fakeHost) listAuth(context.Context) ([]hostAuthFileEntry, error)    { return nil, nil }
func (f *fakeHost) getAuth(context.Context, string) (json.RawMessage, error) { return f.auth, nil }
func (f *fakeHost) getAuthRuntime(context.Context, string) (hostAuthFileEntry, error) {
	return hostAuthFileEntry{}, nil
}
func (f *fakeHost) doHTTP(_ context.Context, request hostHTTPRequest) (hostHTTPResponse, error) {
	f.request = request
	return f.response, nil
}
func (f *fakeHost) log(string, string, map[string]any) {}
