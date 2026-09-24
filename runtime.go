package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

var activeRuntime = newRuntime(cgoHostClient{})

type runtimeState struct {
	mu         sync.Mutex
	cond       *sync.Cond
	cfg        pluginConfig
	snapshot   overviewResponse
	hasSnap    bool
	refreshing bool
	closed     bool
	host       hostClient
}

func newRuntime(host hostClient) *runtimeState {
	r := &runtimeState{host: host, cfg: defaultConfig()}
	r.cond = sync.NewCond(&r.mu)
	return r
}

func (r *runtimeState) applyConfig(cfg pluginConfig) {
	r.mu.Lock()
	r.cfg = cfg
	r.hasSnap = false
	r.mu.Unlock()
}

func (r *runtimeState) shutdown() {
	r.mu.Lock()
	r.closed = true
	r.cond.Broadcast()
	r.mu.Unlock()
}

func (r *runtimeState) handleManagement(req managementRequest) managementResponse {
	if req.Method != http.MethodGet {
		return jsonError(http.StatusMethodNotAllowed, "method_not_allowed", "只支持 GET")
	}
	if req.Path == "/v0/resource/plugins/"+pluginID+panelResourcePath {
		return panelResponse()
	}
	switch strings.TrimPrefix(req.Path, "/v0/management/plugins/"+pluginID) {
	case "/v1/overview":
		force := strings.EqualFold(firstQuery(req.Query, "refresh"), "true")
		snapshot, cached, err := r.getOverview(req.BodyContext(), force)
		if err != nil {
			return jsonError(http.StatusInternalServerError, "overview_failed", err.Error())
		}
		snapshot.Cached = cached
		return jsonResponse(http.StatusOK, snapshot)
	case "/v1/credentials":
		return r.credentials()
	case "/panel":
		return panelResponse()
	default:
		return jsonError(http.StatusNotFound, "not_found", "插件路由不存在")
	}
}

type bodyKey struct{}

func (r managementRequest) BodyContext() context.Context {
	if r.HostCallbackID != "" {
		return context.WithValue(context.Background(), bodyKey{}, r.HostCallbackID)
	}
	return context.Background()
}

func (r *runtimeState) getOverview(ctx context.Context, force bool) (overviewResponse, bool, error) {
	r.mu.Lock()
	defer func() { r.cond.Broadcast() }()
	for {
		if r.closed {
			return overviewResponse{}, false, context.Canceled
		}
		if !force && r.hasSnap && time.Since(r.snapshot.GeneratedAt) < r.cfg.CacheTTL {
			return r.snapshot, true, nil
		}
		if !r.refreshing {
			r.refreshing = true
			cfg := r.cfg
			r.mu.Unlock()
			snapshot := r.refresh(ctx, cfg)
			r.mu.Lock()
			r.refreshing = false
			if snapshot.Summary.Errors == 0 {
				r.snapshot = snapshot
				r.hasSnap = true
			}
			r.cond.Broadcast()
			return snapshot, false, nil
		}
		r.cond.Wait()
		force = false
	}
}

func (r *runtimeState) refresh(ctx context.Context, cfg pluginConfig) overviewResponse {
	out := overviewResponse{GeneratedAt: time.Now().UTC(), CacheTTL: cfg.CacheTTL.String(), Sources: []overviewSource{}}
	var mu sync.Mutex
	var wg sync.WaitGroup
	entries, err := r.host.listAuth(ctx)
	if err == nil {
		selected := make(map[string]struct{}, len(cfg.OAuthAuthIndexes))
		for _, authIndex := range cfg.OAuthAuthIndexes {
			selected[authIndex] = struct{}{}
		}
		for _, entry := range entries {
			if _, exists := selected[entry.AuthIndex]; !exists {
				continue
			}
			if !oauthSupported(normalizedOAuthProvider(entry)) {
				continue
			}
			wg.Add(1)
			go func(entry hostAuthFileEntry) {
				defer wg.Done()
				item := fetchOAuthUsage(ctx, r.host, cfg, entry)
				mu.Lock()
				out.Sources = append(out.Sources, item)
				mu.Unlock()
			}(entry)
		}
	}
	for _, source := range cfg.Sources {
		if !source.Enabled {
			continue
		}
		wg.Add(1)
		go func(source usageSource) {
			defer wg.Done()
			item := fetchSource(ctx, r.host, cfg, source)
			mu.Lock()
			out.Sources = append(out.Sources, item)
			mu.Unlock()
		}(source)
	}
	wg.Wait()
	sort.Slice(out.Sources, func(i, j int) bool { return out.Sources[i].Name < out.Sources[j].Name })
	out.Summary = summarizeOverview(out.Sources)
	return out
}

func summarizeOverview(sources []overviewSource) overviewSummary {
	s := overviewSummary{Total: len(sources)}
	for _, source := range sources {
		s.Items += len(source.Items)
		if source.Status == "ok" {
			s.OK++
		} else {
			s.Errors++
		}
	}
	return s
}

func (r *runtimeState) credentials() managementResponse {
	entries, err := r.host.listAuth(context.Background())
	if err != nil {
		return jsonError(http.StatusInternalServerError, "auth_list_failed", err.Error())
	}
	sort.Slice(entries, func(i, j int) bool { return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name) })
	return jsonResponse(http.StatusOK, map[string]any{"credentials": entries})
}

func firstQuery(query map[string][]string, key string) string {
	if values := query[key]; len(values) > 0 {
		return strings.TrimSpace(values[0])
	}
	return ""
}

func jsonResponse(status int, payload any) managementResponse {
	body, _ := json.Marshal(payload)
	return managementResponse{StatusCode: status, Headers: http.Header{"Content-Type": {"application/json; charset=utf-8"}, "Cache-Control": {"private, no-store"}}, Body: body}
}

func jsonError(status int, code, message string) managementResponse {
	return jsonResponse(status, map[string]any{"error": map[string]any{"code": code, "message": message}})
}
