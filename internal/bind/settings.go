package bind

import (
	"context"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/stmtc233/sportsmeet/internal/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
)

// Settings exposes local preferences and MCP controls only to the desktop UI.
type Settings struct{ *Core }

func (s Settings) Preferences(ctx context.Context) (service.Preferences, error) {
	return s.Svc.Preferences(ctx)
}
func (s Settings) SavePreferences(ctx context.Context, p service.Preferences) error {
	return s.Svc.SavePreferences(ctx, p)
}
func (s Settings) MCPStatus() mcp.Status { return s.MCP.Status() }
func (s Settings) ConfigureMCP(ctx context.Context, enabled bool, port int, rotate bool) error {
	return s.MCP.Configure(ctx, enabled, port, rotate)
}
func (s Settings) RevealToken() string { return s.MCP.Token() }
func (s Settings) CopyToken() error {
	return mygo.Clipboard.Write(transfer.TextData(s.MCP.Token()), mygo.ClipboardOptions{Concealed: true})
}
func (s Settings) CopyConnection(ctx context.Context) error {
	config, err := s.MCP.Connection(ctx)
	if err != nil {
		return err
	}
	return mygo.Clipboard.Write(transfer.TextData(config), mygo.ClipboardOptions{Concealed: true})
}
