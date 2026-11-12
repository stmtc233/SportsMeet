package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/stmtc233/sportsmeet/internal/store"
)

// Preferences contains only settings used by the app. MCP secrets have a separate API.
type Preferences struct {
	Theme         string `json:"theme"`
	SeparateUnits bool   `json:"separateUnits"`
	Balance       bool   `json:"balance"`
	PrintNewPage  bool   `json:"printNewPage"`
}

type MCPConfig struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Token   string `json:"token"`
}

func (s *Service) Preferences(ctx context.Context) (Preferences, error) {
	p := Preferences{Theme: "system", SeparateUnits: true, Balance: true}
	raw, err := s.store.GetSetting(ctx, "preferences")
	if err != nil {
		return p, err
	}
	if raw != "" {
		err = json.Unmarshal([]byte(raw), &p)
	}
	return p, err
}

func (s *Service) SavePreferences(ctx context.Context, p Preferences) error {
	if p.Theme != "system" && p.Theme != "light" && p.Theme != "dark" {
		return invalid("请选择外观")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveSetting(ctx, "preferences", p, "settings.preferences")
}

func (s *Service) MCPConfig(ctx context.Context) (MCPConfig, error) {
	cfg := MCPConfig{Port: 18765}
	raw, err := s.store.GetSetting(ctx, "mcp")
	if err != nil {
		return cfg, err
	}
	if raw != "" {
		err = json.Unmarshal([]byte(raw), &cfg)
	}
	return cfg, err
}

// PrepareMCPConfig validates a local setting and creates a token on first enable.
func PrepareMCPConfig(cfg MCPConfig, rotate bool) (MCPConfig, error) {
	if cfg.Port < 1024 || cfg.Port > 65535 {
		return cfg, invalid("端口应在 1024 到 65535 之间")
	}
	if rotate || cfg.Enabled && cfg.Token == "" {
		var token [32]byte
		if _, err := rand.Read(token[:]); err != nil {
			return cfg, err
		}
		cfg.Token = hex.EncodeToString(token[:])
	}
	return cfg, nil
}

// SaveMCPConfig is called after the runtime has reserved its listener.
func (s *Service) SaveMCPConfig(ctx context.Context, cfg MCPConfig) error {
	checked, err := PrepareMCPConfig(cfg, false)
	if err != nil {
		return err
	}
	if checked != cfg {
		return invalid("请重新开启 MCP")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveSetting(ctx, "mcp", cfg, "settings.mcp")
}

func (s *Service) saveSetting(ctx context.Context, key string, value any, action string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	// Audit stores the key only: MCP tokens must never appear in logs.
	return s.write(ctx, store.Audit{Source: store.SourceUI, Action: action, Message: key}, Change{Kind: "settings"},
		store.Stmt(`INSERT INTO settings(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, string(raw)))
}
