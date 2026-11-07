// Package caddyferretdb is a Caddy plugin that runs FerretDB - the MongoDB-
// compatible database, wekan/FerretDB's v1 fork, over SQLite - inside Caddy, with
// no second process. Part of Weddy (Caddy with the WeKan, Gitea, rclone and
// FerretDB plugins).
//
//   - package store: any plugin opens a database over a directory with
//     store.Acquire - caddy-wekan-plugin gives every domain its own - shared with
//     every other plugin that asks for the same directory;
//
//   - the app "ferretdb": named databases from the Caddyfile, optionally on a TCP
//     address for MongoDB tools (mongodump, mongosh, backups).
//
//     {
//     ferretdb {
//     database wekan /var/lib/weddy/files/domains/wekan.team/files/db {
//     listen 127.0.0.1:27017
//     }
//     }
//     }
package caddyferretdb

import (
	"context"
	"fmt"
	"sort"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"go.uber.org/zap"

	"github.com/wekan/caddy-ferretdb-plugin/store"
)

func init() {
	caddy.RegisterModule(App{})
	httpcaddyfile.RegisterGlobalOption("ferretdb", parseGlobal)
}

// Database is one named database.
type Database struct {
	// Dir is the SQLite directory.
	Dir string `json:"dir"`
	// Listen is a TCP address for MongoDB tools; empty = Weddy's own plugins only.
	Listen string `json:"listen,omitempty"`
}

// App runs the named databases.
type App struct {
	Databases map[string]Database `json:"databases,omitempty"`

	log      *zap.Logger
	releases []func() error
}

// CaddyModule returns the Caddy module information.
func (App) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "ferretdb", New: func() caddy.Module { return new(App) }}
}

// Provision keeps the logger.
func (a *App) Provision(ctx caddy.Context) error {
	a.log = ctx.Logger()
	return nil
}

// Validate checks every database has a directory.
func (a *App) Validate() error {
	for name, d := range a.Databases {
		if d.Dir == "" {
			return fmt.Errorf("ferretdb: database %s has no directory", name)
		}
	}
	return nil
}

// Start opens every database.
func (a *App) Start() error {
	names := make([]string, 0, len(a.Databases))
	for n := range a.Databases {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		d := a.Databases[n]
		_, release, err := store.Acquire(context.Background(), d.Dir,
			&store.Options{TCP: d.Listen, Logger: zapslogLogger(a.log.With(zap.String("database", n)))})
		if err != nil {
			_ = a.Stop()
			return fmt.Errorf("ferretdb: database %s: %w", n, err)
		}
		a.releases = append(a.releases, release)
		a.log.Info("database open", zap.String("database", n), zap.String("dir", d.Dir), zap.String("listen", d.Listen))
	}
	return nil
}

// Stop closes them; a database another plugin still uses stays open for it.
func (a *App) Stop() error {
	var first error
	for _, release := range a.releases {
		if err := release(); err != nil && first == nil {
			first = err
		}
	}
	a.releases = nil
	return first
}

// UnmarshalCaddyfile reads the global option.
func (a *App) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next()
	for d.NextBlock(0) {
		if d.Val() != "database" {
			return d.Errf("unknown ferretdb option %q", d.Val())
		}
		var name, dir string
		if !d.Args(&name, &dir) {
			return d.ArgErr()
		}
		db := Database{Dir: dir}
		for nest := d.Nesting(); d.NextBlock(nest); {
			if d.Val() != "listen" || !d.NextArg() {
				return d.Errf("database %s: only listen <host:port>", name)
			}
			db.Listen = d.Val()
		}
		if a.Databases == nil {
			a.Databases = map[string]Database{}
		}
		if _, dup := a.Databases[name]; dup {
			return d.Errf("database %s twice", name)
		}
		a.Databases[name] = db
	}
	return nil
}

func parseGlobal(d *caddyfile.Dispenser, _ any) (any, error) {
	var a App
	if err := a.UnmarshalCaddyfile(d); err != nil {
		return nil, err
	}
	return httpcaddyfile.App{Name: "ferretdb", Value: caddyconfig.JSON(a, nil)}, nil
}

var (
	_ caddy.App         = (*App)(nil)
	_ caddy.Provisioner = (*App)(nil)
	_ caddy.Validator   = (*App)(nil)
)
