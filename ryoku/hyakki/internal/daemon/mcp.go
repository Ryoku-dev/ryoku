package daemon

import (
	"context"
	"path/filepath"

	"ryoku-i18n"

	"ryoku-rashin/internal/mcpservers"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

func (d *Daemon) mcpStore() (*mcpservers.Store, error) {
	d.infraMu.Lock()
	defer d.infraMu.Unlock()
	if err := d.requireInfra("mcp"); err != nil {
		return nil, err
	}
	if d.infra != nil {
		d.infra.Resume()
		return d.infra, nil
	}
	var v *vault.Vault
	if owner, ok := d.deps.Teams.(interface{ Vault() *vault.Vault }); ok {
		v = owner.Vault()
	}
	s, err := mcpservers.Open(filepath.Join(paths.DataDir(), "mcp-servers.json"), v)
	if err == nil {
		d.infra = s
	}
	return s, err
}

func (d *Daemon) requireInfra(id string) error {
	if !d.feature(id) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Turn this feature on in Settings first."))
	}
	return nil
}

func (d *Daemon) mcpList(ctx context.Context, c *connection, p protocol.MCPListParams) (protocol.MCPListResult, error) {
	if err := d.requireInfra("mcp"); err != nil {
		return protocol.MCPListResult{}, err
	}
	s, err := d.mcpStore()
	if err != nil {
		return protocol.MCPListResult{}, err
	}
	return protocol.MCPListResult{Servers: s.List()}, nil
}

func (d *Daemon) mcpSave(ctx context.Context, c *connection, p protocol.MCPServer) (protocol.MCPServerInfo, error) {
	if err := d.requireInfra("mcp"); err != nil {
		return protocol.MCPServerInfo{}, err
	}
	s, err := d.mcpStore()
	if err != nil {
		return protocol.MCPServerInfo{}, err
	}
	return s.Save(p)
}

func (d *Daemon) mcpDelete(ctx context.Context, c *connection, p protocol.MCPDeleteParams) (protocol.Empty, error) {
	if err := d.requireInfra("mcp"); err != nil {
		return protocol.Empty{}, err
	}
	s, err := d.mcpStore()
	if err != nil {
		return protocol.Empty{}, err
	}
	return protocol.Empty{}, s.Delete(p.ID)
}

func (d *Daemon) mcpCheck(ctx context.Context, c *connection, p protocol.MCPCheckParams) (protocol.MCPServerInfo, error) {
	if err := d.requireInfra("mcp"); err != nil {
		return protocol.MCPServerInfo{}, err
	}
	s, err := d.mcpStore()
	if err != nil {
		return protocol.MCPServerInfo{}, err
	}
	return s.Check(ctx, p.ID)
}

func (d *Daemon) mcpSignIn(ctx context.Context, c *connection, p protocol.MCPSignInParams) (protocol.MCPSignInResult, error) {
	if err := d.requireInfra("mcp"); err != nil {
		return protocol.MCPSignInResult{}, err
	}
	s, err := d.mcpStore()
	if err != nil {
		return protocol.MCPSignInResult{}, err
	}
	d.infraMu.Lock()
	defer d.infraMu.Unlock()
	if err := d.requireInfra("mcp"); err != nil {
		return protocol.MCPSignInResult{}, err
	}
	address, err := s.SignIn(ctx, p.ID)
	return protocol.MCPSignInResult{URL: address}, err
}

func (d *Daemon) cancelMCPSignIns() {
	d.infraMu.Lock()
	defer d.infraMu.Unlock()
	if d.infra != nil {
		d.infra.CancelSignIns()
	}
}
