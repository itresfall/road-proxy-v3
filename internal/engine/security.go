package engine

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type websocketSecurityState struct {
	mu            sync.Mutex
	active        int
	activeByIP    map[string]int
	rateByIP      map[string]*websocketRateWindow
	lastRatePrune time.Time
}

type websocketRateWindow struct {
	start time.Time
	count int
}

const websocketRateWindowTTL = 2 * time.Minute

func (e *Engine) admitWebSocket(r *http.Request) (func(), int, string) {
	clientIP := e.clientIP(r)
	now := time.Now()

	e.wsSecurity.mu.Lock()
	defer e.wsSecurity.mu.Unlock()

	if limit := e.cfg.HTTP.RateLimitPerMinute; limit > 0 {
		e.pruneWebSocketRateWindowsLocked(now)
		window := e.wsSecurity.rateByIP[clientIP]
		if window == nil || now.Sub(window.start) >= time.Minute {
			window = &websocketRateWindow{start: now}
			e.wsSecurity.rateByIP[clientIP] = window
		}
		window.count++
		if window.count > limit {
			return nil, http.StatusTooManyRequests, "websocket rate limit exceeded"
		}
	}

	if limit := e.cfg.HTTP.MaxConnections; limit > 0 && e.wsSecurity.active >= limit {
		return nil, http.StatusServiceUnavailable, "websocket connection limit exceeded"
	}
	if limit := e.cfg.HTTP.MaxConnectionsPerIP; limit > 0 && e.wsSecurity.activeByIP[clientIP] >= limit {
		return nil, http.StatusTooManyRequests, "websocket per-ip connection limit exceeded"
	}

	e.wsSecurity.active++
	e.wsSecurity.activeByIP[clientIP]++

	released := false
	return func() {
		e.wsSecurity.mu.Lock()
		defer e.wsSecurity.mu.Unlock()
		if released {
			return
		}
		released = true
		if e.wsSecurity.active > 0 {
			e.wsSecurity.active--
		}
		if e.wsSecurity.activeByIP[clientIP] <= 1 {
			delete(e.wsSecurity.activeByIP, clientIP)
			return
		}
		e.wsSecurity.activeByIP[clientIP]--
	}, http.StatusOK, ""
}

func (e *Engine) pruneWebSocketRateWindowsLocked(now time.Time) {
	if e.wsSecurity.lastRatePrune.IsZero() {
		e.wsSecurity.lastRatePrune = now
		return
	}
	if now.Sub(e.wsSecurity.lastRatePrune) < time.Minute {
		return
	}
	e.wsSecurity.lastRatePrune = now
	for ip, window := range e.wsSecurity.rateByIP {
		if window == nil || now.Sub(window.start) >= websocketRateWindowTTL {
			delete(e.wsSecurity.rateByIP, ip)
		}
	}
}

func (e *Engine) hostAllowed(r *http.Request) bool {
	allowedHosts := e.cfg.HTTP.AllowedHosts
	if len(allowedHosts) == 0 {
		return true
	}
	return hostMatches(hostWithoutPort(r.Host), allowedHosts)
}

func (e *Engine) checkWebSocketOrigin(r *http.Request) bool {
	if !e.hostAllowed(r) {
		return false
	}
	allowedOrigins := e.cfg.HTTP.AllowedOrigins
	if len(allowedOrigins) == 0 {
		return true
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return hostMatches(hostWithoutPort(parsed.Host), allowedOrigins) || originMatches(origin, allowedOrigins)
}

func (e *Engine) hostAllowMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !e.hostAllowed(r) {
			e.stats.IncError()
			http.Error(w, "host not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (e *Engine) allowPluginAPI(w http.ResponseWriter) bool {
	if e.cfg.Control.PluginAPIPublic || e.cfg.WSAuthEnabled() {
		return true
	}
	http.Error(w, "plugin api is private", http.StatusForbidden)
	return false
}

func (e *Engine) clientIP(r *http.Request) string {
	if e.cfg.HTTP.TrustProxyHeaders {
		if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
			return ip
		}
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			first := strings.TrimSpace(strings.Split(xff, ",")[0])
			if first != "" {
				return first
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func hostMatches(host string, allowed []string) bool {
	host = strings.ToLower(hostWithoutPort(host))
	for _, pattern := range allowed {
		pattern = strings.ToLower(hostWithoutPort(pattern))
		switch {
		case pattern == "*":
			return true
		case pattern == host:
			return true
		case strings.HasPrefix(pattern, "*.") && strings.HasSuffix(host, strings.TrimPrefix(pattern, "*")):
			return true
		}
	}
	return false
}

func originMatches(origin string, allowed []string) bool {
	origin = strings.TrimRight(strings.ToLower(strings.TrimSpace(origin)), "/")
	for _, pattern := range allowed {
		pattern = strings.TrimRight(strings.ToLower(strings.TrimSpace(pattern)), "/")
		if pattern == "*" || pattern == origin {
			return true
		}
	}
	return false
}

func hostWithoutPort(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		if parsed, err := url.Parse(raw); err == nil {
			raw = parsed.Host
		}
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(raw, "[]")
}

func websocketLimitLogMessage(status int, reason string) string {
	if reason == "" {
		return fmt.Sprintf("websocket admission rejected with status %d", status)
	}
	return reason
}
