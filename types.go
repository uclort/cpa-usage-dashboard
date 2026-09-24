package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

const (
	pluginID      = "cpa-usage-dashboard"
	pluginVersion = "0.4.0"

	panelResourcePath = "/panel"
	overviewRoute     = "/plugins/cpa-usage-dashboard/v1/overview"
	credentialsRoute  = "/plugins/cpa-usage-dashboard/v1/credentials"
)

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
}

type registration struct {
	SchemaVersion uint32                   `json:"schema_version"`
	Metadata      metadata                 `json:"metadata"`
	Capabilities  registrationCapabilities `json:"capabilities"`
}

type registrationCapabilities struct {
	ManagementAPI bool `json:"management_api"`
	UsagePlugin   bool `json:"usage_plugin"`
}

type metadata struct {
	Name             string        `json:"Name"`
	Version          string        `json:"Version"`
	Author           string        `json:"Author"`
	GitHubRepository string        `json:"GitHubRepository"`
	Logo             string        `json:"Logo"`
	ConfigFields     []configField `json:"ConfigFields"`
}

type configField struct {
	Name        string `json:"Name"`
	Type        string `json:"Type"`
	Description string `json:"Description"`
}

type managementRegistrationResponse struct {
	Routes    []managementRoute `json:"routes,omitempty"`
	Resources []resourceRoute   `json:"resources,omitempty"`
}

type resourceRoute struct {
	Path        string `json:"Path"`
	Menu        string `json:"Menu"`
	Description string `json:"Description,omitempty"`
}

type managementRoute struct {
	Method      string `json:"Method"`
	Path        string `json:"Path"`
	Menu        string `json:"Menu"`
	Description string `json:"Description,omitempty"`
}

type managementRequest struct {
	Method         string
	Path           string
	Headers        http.Header
	Query          url.Values
	Body           []byte
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

type managementResponse struct {
	StatusCode int         `json:"StatusCode"`
	Headers    http.Header `json:"Headers"`
	Body       []byte
}

type usageSource struct {
	ID           string            `json:"id" yaml:"id"`
	Name         string            `json:"name" yaml:"name"`
	Enabled      bool              `json:"enabled" yaml:"enabled"`
	Method       string            `json:"method" yaml:"method"`
	URL          string            `json:"url" yaml:"url"`
	Headers      map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body         string            `json:"body,omitempty" yaml:"body,omitempty"`
	ListPath     string            `json:"list_path" yaml:"list_path"`
	FieldTotal   string            `json:"field_total" yaml:"field_total"`
	FieldUsed    string            `json:"field_used" yaml:"field_used"`
	FieldName    string            `json:"field_name,omitempty" yaml:"field_name,omitempty"`
	FieldStatus  string            `json:"field_status,omitempty" yaml:"field_status,omitempty"`
	FieldResetAt string            `json:"field_reset_at,omitempty" yaml:"field_reset_at,omitempty"`
	Divisor      float64           `json:"divisor,omitempty" yaml:"divisor,omitempty"`
	Multiplier   float64           `json:"multiplier,omitempty" yaml:"multiplier,omitempty"`
	Unit         string            `json:"unit,omitempty" yaml:"unit,omitempty"`
	AuthMode     string            `json:"auth_mode,omitempty" yaml:"auth_mode,omitempty"`
	CustomToken  string            `json:"custom_token,omitempty" yaml:"custom_token,omitempty"`
	AuthIndex    string            `json:"auth_index,omitempty" yaml:"auth_index,omitempty"`
	AuthField    string            `json:"auth_field,omitempty" yaml:"auth_field,omitempty"`
	AuthHeader   string            `json:"auth_header,omitempty" yaml:"auth_header,omitempty"`
	AuthPrefix   string            `json:"auth_prefix,omitempty" yaml:"auth_prefix,omitempty"`
}

type overviewSource struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Status    string          `json:"status"`
	Items     []usageItem     `json:"items,omitempty"`
	Error     *usageItemError `json:"error,omitempty"`
	FetchedAt time.Time       `json:"fetched_at,omitempty"`
}

type usageItem struct {
	Name        string     `json:"name"`
	Unit        string     `json:"unit,omitempty"`
	Total       float64    `json:"total,omitempty"`
	Used        float64    `json:"used"`
	Remaining   float64    `json:"remaining"`
	UsedPercent float64    `json:"used_percent"`
	ResetAt     *time.Time `json:"reset_at,omitempty"`
}

type usageItemError struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	UpstreamStatus int    `json:"upstream_status,omitempty"`
}

type overviewResponse struct {
	GeneratedAt time.Time        `json:"generated_at"`
	Cached      bool             `json:"cached"`
	CacheTTL    string           `json:"cache_ttl"`
	Summary     overviewSummary  `json:"summary"`
	Sources     []overviewSource `json:"sources"`
}

type overviewSummary struct {
	Total  int `json:"total"`
	OK     int `json:"ok"`
	Errors int `json:"errors"`
	Items  int `json:"items"`
}
