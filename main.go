// SPDX-FileCopyrightText: 2026 Dohna <pub@lya.moe>
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	_ "github.com/caddy-dns/cloudflare"
	_ "github.com/caddyserver/cache-handler"
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/homonyms/caddy-cf/plugins"
)

func main() {
	caddycmd.Main()
}
