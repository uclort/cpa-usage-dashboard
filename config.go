package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type lifecycleRequest struct {
	ConfigYAML json.RawMessage `json:"config_yaml"`
}

type pluginConfig struct {
	AutoRefresh      time.Duration
	RequestTimeout   time.Duration
	Sources          []usageSource
	OAuthAuthIndexes []string
	Order            []string
}

type rawConfig struct {
	AutoRefresh      string        `yaml:"auto-refresh"`
	RequestTimeout   string        `yaml:"request-timeout"`
	Sources          []usageSource `yaml:"sources"`
	OAuthAuthIndexes []string      `yaml:"oauth-auth-indexes"`
	Order            []string      `yaml:"order"`
}

func defaultConfig() pluginConfig {
	return pluginConfig{AutoRefresh: 5 * time.Minute, RequestTimeout: 15 * time.Second}
}

func decodeLifecycleConfig(raw []byte) (pluginConfig, error) {
	cfg := defaultConfig()
	if len(raw) == 0 {
		return cfg, nil
	}
	var req lifecycleRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return cfg, fmt.Errorf("解析插件配置失败：%w", err)
	}
	text, err := lifecycleConfigText(req.ConfigYAML)
	if err != nil {
		return cfg, err
	}
	var parsed rawConfig
	if err := yaml.Unmarshal([]byte(text), &parsed); err != nil {
		return cfg, fmt.Errorf("解析 YAML 配置失败：%w", err)
	}
	if parsed.AutoRefresh != "" {
		value, err := time.ParseDuration(parsed.AutoRefresh)
		if err != nil || value < 0 {
			return cfg, fmt.Errorf("auto-refresh 必须是 0 或有效时长")
		}
		cfg.AutoRefresh = value
	}
	if parsed.RequestTimeout != "" {
		value, err := time.ParseDuration(parsed.RequestTimeout)
		if err != nil || value < time.Second || value > time.Minute {
			return cfg, fmt.Errorf("request-timeout 必须在 1s 到 1m 之间")
		}
		cfg.RequestTimeout = value
	}
	cfg.Sources = parsed.Sources
	cfg.OAuthAuthIndexes = parsed.OAuthAuthIndexes
	cfg.Order = parsed.Order
	return cfg, nil
}

func lifecycleConfigText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if decoded, decodeErr := base64.StdEncoding.DecodeString(text); decodeErr == nil && strings.Contains(string(decoded), ":") {
			return string(decoded), nil
		}
		return text, nil
	}
	var bytes []byte
	if err := json.Unmarshal(raw, &bytes); err == nil {
		return string(bytes), nil
	}
	return "", fmt.Errorf("config_yaml 必须是字符串或字节数组")
}
