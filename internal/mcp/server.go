// Package mcp serves authenticated MCP on IPv4 loopback. All data access uses service.
package mcp

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
)

const MaxBody = 2 << 20

type Status struct {
	Enabled bool   `json:"enabled"`
	Running bool   `json:"running"`
	Port    int    `json:"port"`
	Address string `json:"address"`
	Error   string `json:"error"`
}

// Manager owns the listener, authentication and its persisted configuration.
// updateMu serializes settings changes; gate prevents token rotation racing admitted calls.
type Manager struct {
	svc       *service.Service
	updateMu  sync.Mutex
	gate      sync.RWMutex
	config    service.MCPConfig
	server    *http.Server
	handler   http.Handler
	lastError string
}

func New(svc *service.Service) *Manager {
	m := &Manager{svc: svc}
	server := NewServer(svc)
	m.handler = sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	return m
}

// Start restores the desired state. A busy port leaves the desktop app usable.
func (m *Manager) Start(ctx context.Context) error {
	m.updateMu.Lock()
	defer m.updateMu.Unlock()
	cfg, err := m.svc.MCPConfig(ctx)
	if err != nil {
		return err
	}
	m.gate.Lock()
	defer m.gate.Unlock()
	m.config = cfg
	if !cfg.Enabled {
		return nil
	}
	if cfg.Token == "" {
		m.lastError = "请重新开启 MCP"
		return errors.New(m.lastError)
	}
	srv, listener, err := m.reserve(cfg.Port)
	if err != nil {
		m.lastError = "端口无法监听，请更换端口"
		return errors.New(m.lastError)
	}
	m.server = srv
	go func() { _ = srv.Serve(listener) }()
	return nil
}

// Configure reserves a new port before persisting. On any failure the old listener and settings survive.
func (m *Manager) Configure(ctx context.Context, enabled bool, port int, rotate bool) error {
	m.updateMu.Lock()
	defer m.updateMu.Unlock()
	old, err := m.svc.MCPConfig(ctx)
	if err != nil {
		return err
	}
	cfg, err := service.PrepareMCPConfig(service.MCPConfig{Enabled: enabled, Port: port, Token: old.Token}, rotate)
	if err != nil {
		return err
	}
	m.gate.Lock()
	srv := m.server
	var replacement *http.Server
	var reserved net.Listener
	if cfg.Enabled && (srv == nil || m.config.Port != cfg.Port) {
		replacement, reserved, err = m.reserve(cfg.Port)
		if err != nil {
			m.gate.Unlock()
			return errors.New("端口无法监听，请更换端口")
		}
	}
	if err = m.svc.SaveMCPConfig(ctx, cfg); err != nil {
		if reserved != nil {
			_ = reserved.Close()
		}
		m.gate.Unlock()
		return err
	}
	m.config, m.lastError = cfg, ""
	if !cfg.Enabled {
		m.server = nil
	} else if replacement != nil {
		m.server = replacement
		go func() { _ = replacement.Serve(reserved) }()
	}
	m.gate.Unlock()
	if srv != nil && (!cfg.Enabled || replacement != nil) {
		_ = srv.Close()
	}
	return nil
}

// reserve opens the socket without serving requests until persistence succeeds.
func (m *Manager) reserve(port int) (*http.Server, net.Listener, error) {
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, nil, err
	}
	srv := &http.Server{Handler: m, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	return srv, listener, nil
}

func (m *Manager) Status() Status {
	m.gate.RLock()
	defer m.gate.RUnlock()
	return Status{Enabled: m.config.Enabled, Running: m.server != nil, Port: m.config.Port,
		Address: fmt.Sprintf("http://127.0.0.1:%d/mcp", m.config.Port), Error: m.lastError}
}

func (m *Manager) Connection(ctx context.Context) (string, error) {
	m.gate.RLock()
	defer m.gate.RUnlock()
	if !m.config.Enabled || m.server == nil {
		return "", errors.New("请先开启 MCP")
	}
	data, err := json.MarshalIndent(map[string]any{"mcpServers": map[string]any{"sportsmeet": map[string]any{
		"url": fmt.Sprintf("http://127.0.0.1:%d/mcp", m.config.Port), "headers": map[string]string{"Authorization": "Bearer " + m.config.Token},
	}}}, "", "  ")
	return string(data), err
}

func (m *Manager) Token() string {
	m.gate.RLock()
	defer m.gate.RUnlock()
	return m.config.Token
}

func (m *Manager) Close() error {
	m.updateMu.Lock()
	defer m.updateMu.Unlock()
	m.gate.Lock()
	srv := m.server
	m.server = nil
	m.gate.Unlock()
	if srv != nil {
		return srv.Close()
	}
	return nil
}

func localHost(host string) bool { return host == "localhost" || host == "127.0.0.1" || host == "::1" }

func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.gate.RLock()
	defer m.gate.RUnlock()
	w.Header().Set("Cache-Control", "no-store")
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if !localHost(host) {
		http.Error(w, "forbidden host", http.StatusForbidden)
		return
	}
	if origins, present := r.Header["Origin"]; present {
		if len(origins) != 1 {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		u, err := url.Parse(origins[0])
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !localHost(u.Hostname()) || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
	}
	if !m.config.Enabled || m.server == nil {
		http.Error(w, "disabled", http.StatusServiceUnavailable)
		return
	}
	want := "Bearer " + m.config.Token
	if m.config.Token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(want)) != 1 {
		w.Header().Set("WWW-Authenticate", `Bearer realm="SportsMeet"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.URL.Path != "/mcp" {
		http.NotFound(w, r)
		return
	}
	if r.Body != nil {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBody))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			} else {
				http.Error(w, "invalid request", http.StatusBadRequest)
			}
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
	}
	m.handler.ServeHTTP(w, r)
}
