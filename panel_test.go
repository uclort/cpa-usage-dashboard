package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPanelScriptAndCPAMPTheme(t *testing.T) {
	for _, want := range []string{
		`path.replace(/^\/+/,"")`,
		`--dash-primary:var(--primary-color,#409eff)`,
		`.loading{display:grid;place-items:center;min-height:420px}`,
		`toggle-custom-token`,
		`用于访问用量查询接口的独立 Token`,
		`用量源管理`,
		`OAuth 凭证`,
		`enc::v2::`,
		`oauth-auth-indexes`,
		`自定义查询 Token`,
		`绑定已有凭证`,
	} {
		if !strings.Contains(panelDocument, want) {
			t.Fatalf("panel missing %q", want)
		}
	}
	if strings.Contains(panelDocument, `path.replace(/^\\/+/,"")`) {
		t.Fatal("panel contains invalid escaped regular expression")
	}
}

func TestRuntimeDoesNotDeadlockAfterRefresh(t *testing.T) {
	r := newRuntime(&fakeHost{})
	r.applyConfig(pluginConfig{CacheTTL: time.Minute, RequestTimeout: time.Second})
	for i := 0; i < 3; i++ {
		if _, _, err := r.getOverview(context.Background(), true); err != nil {
			t.Fatalf("getOverview %d: %v", i, err)
		}
	}
}
