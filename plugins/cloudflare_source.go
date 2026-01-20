// SPDX-FileCopyrightText: 2026 Dohna <pub@lya.moe>
// SPDX-FileCopyrightText: 2026 WeidiDeng @ GitHub
//
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"bufio"
	"context"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

const (
	ipv4 = "https://www.cloudflare.com/ips-v4"
	ipv6 = "https://www.cloudflare.com/ips-v6"
)

func init() {
	caddy.RegisterModule(CloudflareIPRange{})
}

type CloudflareIPRange struct {
	Interval caddy.Duration `json:"interval,omitempty"`
	Timeout  caddy.Duration `json:"timeout,omitempty"`

	ranges []netip.Prefix
	ctx    caddy.Context
	lock   *sync.RWMutex
	logger *zap.Logger
}

func (CloudflareIPRange) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.ip_sources.cloudflare",
		New: func() caddy.Module { return new(CloudflareIPRange) },
	}
}

func (s *CloudflareIPRange) Provision(ctx caddy.Context) error {
	s.ctx = ctx
	s.logger = ctx.Logger()
	s.lock = new(sync.RWMutex)

	s.logger.Info("fetching initial cloudflare ip ranges...")
	initialRanges, err := s.getPrefixes()
	if err != nil {
		s.logger.Error("failed to fetch initial cloudflare ips", zap.Error(err))
	} else {
		s.ranges = initialRanges
		s.logger.Info("cloudflare ip ranges loaded successfully", zap.Int("count", len(s.ranges)))
	}

	go s.refreshLoop()
	return nil
}

func (s *CloudflareIPRange) refreshLoop() {
	if s.Interval == 0 {
		s.Interval = caddy.Duration(time.Hour)
	}
	ticker := time.NewTicker(time.Duration(s.Interval))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			newRanges, err := s.getPrefixes()
			if err != nil {
				s.logger.Warn("could not refresh cloudflare ips", zap.Error(err))
				continue
			}
			s.lock.Lock()
			s.ranges = newRanges
			s.lock.Unlock()
			s.logger.Debug("cloudflare ip ranges refreshed", zap.Int("count", len(s.ranges)))
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *CloudflareIPRange) getPrefixes() ([]netip.Prefix, error) {
	var full []netip.Prefix
	for _, api := range []string{ipv4, ipv6} {
		prefixes, err := s.fetch(api)
		if err != nil {
			return nil, err
		}
		full = append(full, prefixes...)
	}
	return full, nil
}

func (s *CloudflareIPRange) fetch(api string) ([]netip.Prefix, error) {
	timeout := time.Duration(s.Timeout)
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(s.ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var prefixes []netip.Prefix
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(line)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

func (s *CloudflareIPRange) GetIPRanges(_ *http.Request) []netip.Prefix {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.ranges
}

func (m *CloudflareIPRange) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // skip "cloudflare"
	for nesting := d.Nesting(); d.NextBlock(nesting); {
		switch d.Val() {
		case "interval":
			if !d.NextArg() {
				return d.ArgErr()
			}
			dur, err := caddy.ParseDuration(d.Val())
			if err != nil {
				return err
			}
			m.Interval = caddy.Duration(dur)
		case "timeout":
			if !d.NextArg() {
				return d.ArgErr()
			}
			dur, err := caddy.ParseDuration(d.Val())
			if err != nil {
				return err
			}
			m.Timeout = caddy.Duration(dur)
		}
	}
	return nil
}

// interface guards
var (
	_ caddy.Module            = (*CloudflareIPRange)(nil)
	_ caddy.Provisioner       = (*CloudflareIPRange)(nil)
	_ caddyfile.Unmarshaler   = (*CloudflareIPRange)(nil)
	_ caddyhttp.IPRangeSource = (*CloudflareIPRange)(nil)
)
