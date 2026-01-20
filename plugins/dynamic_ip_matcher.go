// SPDX-FileCopyrightText: 2026 Dohna <pub@lya.moe>
// SPDX-FileCopyrightText: 2026 lanrat @ GitHub
//
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(MatchDynamicRemoteIP{})
}

type MatchDynamicRemoteIP struct {
	ProvidersRaw json.RawMessage         `json:"providers,omitempty" caddy:"namespace=http.ip_sources inline_key=source"`
	Providers    caddyhttp.IPRangeSource `json:"-"`
	logger       *zap.Logger
}

func (MatchDynamicRemoteIP) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.matchers.dynamic_remote_ip",
		New: func() caddy.Module { return new(MatchDynamicRemoteIP) },
	}
}

func (m *MatchDynamicRemoteIP) Provision(ctx caddy.Context) error {
	m.logger = ctx.Logger()
	if m.ProvidersRaw != nil {
		val, err := ctx.LoadModule(m, "ProvidersRaw")
		if err != nil {
			return err
		}
		m.Providers = val.(caddyhttp.IPRangeSource)
	}
	return nil
}

func (m MatchDynamicRemoteIP) Match(r *http.Request) bool {
	address := r.RemoteAddr
	remoteIP, err := parseIPZoneFromString(address)
	if err != nil {
		m.logger.Error("getting remote IP", zap.Error(err))
		return false
	}

	if m.Providers == nil {
		return false
	}

	cidrs := m.Providers.GetIPRanges(r)
	for _, ipRange := range cidrs {
		if ipRange.Contains(remoteIP) {
			return true
		}
	}
	return false
}

func parseIPZoneFromString(address string) (netip.Addr, error) {
	ipStr, _, err := net.SplitHostPort(address)
	if err != nil {
		ipStr = address
	}
	ipStr, _, _ = strings.Cut(ipStr, "%")
	return netip.ParseAddr(ipStr)
}

func (m *MatchDynamicRemoteIP) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next()
	if !d.NextArg() {
		return d.ArgErr()
	}
	dynModule := d.Val()
	modID := "http.ip_sources." + dynModule
	mod, err := caddyfile.UnmarshalModule(d, modID)
	if err != nil {
		return err
	}
	provider, ok := mod.(caddyhttp.IPRangeSource)
	if !ok {
		return d.Errf("module %s is not an IPRangeSource", modID)
	}
	m.ProvidersRaw = caddyconfig.JSONModuleObject(provider, "source", dynModule, nil)
	return nil
}

// Interface guards
var (
	_ caddy.Module             = (*MatchDynamicRemoteIP)(nil)
	_ caddy.Provisioner        = (*MatchDynamicRemoteIP)(nil)
	_ caddyfile.Unmarshaler    = (*MatchDynamicRemoteIP)(nil)
	_ caddyhttp.RequestMatcher = (*MatchDynamicRemoteIP)(nil)
)
