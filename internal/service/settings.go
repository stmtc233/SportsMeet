package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/stmtc233/sportsmeet/internal/store"
)

// Preferences are the app's own settings; a meet's arrange and print
// settings are its MeetSettings. MCP secrets have a separate API.
type Preferences struct {
	Theme string `json:"theme"`
	// UpdatePrereleases includes test versions in mygo's update checks.
	UpdatePrereleases bool `json:"updatePrereleases"`
}

type MCPConfig struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Token   string `json:"token"`
}

func (s *Service) Preferences(ctx context.Context) (Preferences, error) {
	p := Preferences{Theme: "system"}
	raw, err := s.store.GetSetting(ctx, "preferences")
	if err != nil || raw == "" {
		return p, err
	}
	return p, json.Unmarshal([]byte(raw), &p)
}

// Where a per-heat sheet starts a new page.
const (
	SheetBreakNone  = "none"  // only when a page is full
	SheetBreakRound = "round" // each round: its heats share pages
	SheetBreakHeat  = "heat"  // each heat
)

func validSheetBreak(v string) bool {
	return v == SheetBreakNone || v == SheetBreakRound || v == SheetBreakHeat
}

// sheetBreaks marks where the sheets of list start a new page.
func sheetBreaks(list []sheetHeat, mode string) []bool {
	out := make([]bool, len(list))
	for i := 1; i < len(list); i++ {
		out[i] = mode == SheetBreakHeat || mode == SheetBreakRound && list[i].round.ID != list[i-1].round.ID
	}
	return out
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
