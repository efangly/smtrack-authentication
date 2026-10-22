// Package migrations embeds the goose SQL migrations for the tenant/department schema.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
