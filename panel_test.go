package main

import (
	"strings"
	"testing"
)

func TestPanelScriptAndCPAMPTheme(t *testing.T) {
	for _, want := range []string{
		`path.replace(/^\/+/,"")`,
		`--dash-primary:var(--primary-color,#409eff)`,
		`用量源管理`,
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
