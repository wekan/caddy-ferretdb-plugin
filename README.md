# Caddy plugin: FerretDB

A [Caddy](https://caddyserver.com) plugin that runs [FerretDB](https://www.ferretdb.com) -
the MongoDB-compatible database, as [wekan/FerretDB](https://github.com/wekan/FerretDB)'s
v1 fork over SQLite - **inside Caddy**, with no second process. Part of
[Weddy](https://github.com/wekan/weddy): Caddy with the WeKan, Gitea, rclone and FerretDB
plugins.

## For other plugins: package store

```go
db, release, err := store.Acquire(ctx, "/var/lib/weddy/files/domains/wekan.team/files/db", nil)
defer release()
db.DB.Collection("boards").Find(ctx, bson.M{})   // the official MongoDB Go driver
```

A database is a SQLite directory: WeKan's database `wekan` is `wekan.sqlite` there, the
same file Node.js WeKan with FerretDB writes, so the data moves between them as it is.
FerretDB listens on a Unix socket only Weddy knows. `Acquire` shares one FerretDB per
directory between every plugin that asks for it - two FerretDBs never write the same
SQLite files - and the last release closes it.
[caddy-wekan-plugin](https://github.com/wekan/caddy-wekan-plugin) opens one per domain.

## The app: named databases from the Caddyfile

```caddyfile
{
	ferretdb {
		database wekan /var/lib/weddy/files/domains/wekan.team/files/db {
			listen 127.0.0.1:27017    # optional: for mongodump, mongosh, backups
		}
	}
}
```

Without `listen`, a database is reached only by Weddy's own plugins.

## Building and tests

wekan/FerretDB is checked out beside this plugin (`go.mod` replaces upstream FerretDB
with `../FerretDB`), as in WeKan's `.tools/w/` and Weddy's build.

```sh
CGO_ENABLED=0 go test ./...
```

A real Caddy serves a database on TCP to a MongoDB client; the store is written,
closed, reopened from `wekan.sqlite` and read back; and shared between two holders and
closed by the last.

## License

This plugin is MIT - see [LICENSE](LICENSE). FerretDB is Apache-2.0.

Maintainer and contributor rules are in
[WeKan's CLAUDE.md and AGENTS.md](https://github.com/wekan/wekan), which cover every
repository WeKan clones into its `.tools/` directory.
