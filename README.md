# goway

[![Go Reference](https://pkg.go.dev/badge/github.com/cgardev/goway.svg)](https://pkg.go.dev/github.com/cgardev/goway)
[![Go Report Card](https://goreportcard.com/badge/github.com/cgardev/goway)](https://goreportcard.com/report/github.com/cgardev/goway)
[![CI](https://github.com/cgardev/goway/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/cgardev/goway/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/cgardev/goway)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Status: Alpha](https://img.shields.io/badge/status-alpha-orange.svg)](#status-and-roadmap)

A version-based database schema migration library for Go, inspired by the
behavior and configuration model of [Flyway](https://flywaydb.org/). It discovers
versioned and repeatable SQL migration scripts, records what has been applied in
a schema history table, and brings a database up to date by running the pending
scripts in order, each inside its own transaction.

> **Alpha.** The API may still change and there are no tagged releases yet. Pin a
> specific commit when depending on this module.

## Why

Go has excellent low level database tooling, but migrating a schema reliably and
reproducibly across environments is usually delegated to an external binary or a
heavyweight framework. goway brings the well understood migration model —
immutable, ordered, checksummed migrations recorded in a history table — to Go as
a small, embeddable package that uses the connection pool the application already
owns.

- **Zero dependency core.** The library depends only on the standard library.
  Database access goes through `database/sql`, so the application supplies its own
  driver and `*sql.DB`.
- **Two databases, no C.** PostgreSQL through
  [`github.com/jackc/pgx/v5`](https://github.com/jackc/pgx) and SQLite through the
  pure Go driver [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite),
  which needs no `cgo`.
- **Drop-in friendly.** Migration naming, the CRC32 checksum algorithm, version
  comparison, the schema history table (named `flyway_schema_history` by default),
  and SQL statement splitting follow Flyway's documented semantics, so an existing
  migration set and an existing history table behave as expected.

## Install

```sh
go get github.com/cgardev/goway
```

## Quick start

Migrations can be read from the file system or embedded into the binary with
`go:embed`.

```go
package main

import (
	"context"
	"database/sql"
	"embed"
	"log"

	"github.com/cgardev/goway"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed db/migration/*.sql
var migrations embed.FS

func main() {
	database, err := sql.Open("pgx", "postgres://user:password@localhost:5432/app")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	migrator, err := goway.Configure().
		DataSource(database).
		FS(migrations, "db/migration").
		Schemas("public").
		CreateSchemas(true).
		Load()
	if err != nil {
		log.Fatal(err)
	}

	result, err := migrator.Migrate(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("applied %d migration(s); now at version %s",
		result.MigrationsExecuted, result.TargetSchemaVersion)
}
```

The dialect is detected automatically from the connection; call `Dialect` to set
it explicitly and skip detection.

## Migration scripts

Scripts follow the same naming convention as Flyway:

| Kind       | Pattern                         | Example                |
| ---------- | ------------------------------- | ---------------------- |
| Versioned  | `V<version>__<description>.sql`  | `V1__create_users.sql` |
| Versioned  | dotted versions allowed          | `V2.1__add_index.sql`  |
| Repeatable | `R__<description>.sql`           | `R__active_users.sql`  |

Versioned migrations run once, in ascending version order. Repeatable migrations
run after all versioned ones and are re-applied whenever their checksum changes.
A migration may contain several statements; they are split on the semicolon
delimiter, taking into account quoted strings, comments, PostgreSQL dollar quoted
bodies, and SQLite trigger blocks.

## Commands

Every command is a method on the loaded `*goway.Flyway` value and takes a
`context.Context`.

| Command    | Description                                                              |
| ---------- | ----------------------------------------------------------------------- |
| `Migrate`  | Applies every pending migration in order.                               |
| `Info`     | Reports the state of every migration without changing the database.     |
| `Validate` | Checks applied migrations against the resolved scripts.                  |
| `Baseline` | Records a baseline so an existing database can be brought under control. |
| `Repair`   | Removes failed entries and realigns recorded checksums.                 |
| `Clean`    | Drops every object in the managed schemas. Disabled by default.         |

## Command line tool

A command line front end lives in the `cmd/goway` module.

```sh
go run github.com/cgardev/goway/cmd/goway \
  -url 'postgres://user:password@localhost:5432/app' \
  -locations filesystem:db/migration \
  migrate
```

It selects the driver from the URL scheme: `postgres://` (or `postgresql://`)
uses pgx, and `sqlite:` (or `file:`) uses the pure Go SQLite driver. Run it with
no command to see all flags.

## Dialects

| Capability                | PostgreSQL | SQLite |
| ------------------------- | ---------- | ------ |
| Addressable schemas       | yes        | no     |
| Transactional migrations  | yes        | yes    |
| Dollar quoted bodies      | yes        | n/a    |
| Trigger block splitting   | n/a        | yes    |
| Advisory migration lock   | yes        | n/a    |

Only the two most recent major versions of each database are targeted.

## Testing

The core library and its unit tests use only the standard library:

```sh
go test ./... -count=1
```

The `integration` module exercises real databases. The SQLite tests run anywhere
through the pure Go driver; the PostgreSQL tests start a container with
testcontainers and skip themselves when Docker is unavailable:

```sh
go -C integration test ./... -count=1
```

## Status and roadmap

Implemented: versioned and repeatable SQL migrations, the schema history table,
migrate, info, validate, baseline, repair and clean, placeholder replacement,
multiple locations, embedded file systems, schema creation, and a command line
tool.

Not yet implemented: Java style code based migrations, lifecycle callbacks, undo
migrations, and listing every historical run of a repeatable migration (only the
latest run is reported).

## Acknowledgements and License

goway is an independent, clean-room implementation inspired by the behavior and
public configuration model of Flyway. It is not affiliated with or endorsed by
the Flyway project or Red Gate Software Ltd. No Flyway source code is used in this
project; goway is built from scratch in Go and references only the publicly
documented behavior and public API surface of Flyway.

Flyway is a trademark of Red Gate Software Ltd. All references to Flyway are for
identification and comparison purposes only. For interoperability, goway uses the
same default schema history table name (`flyway_schema_history`) and the same
`${flyway:...}` placeholder names; these are technical defaults and remain
configurable.

goway is distributed under the MIT License (see [LICENSE](LICENSE)). Flyway
Community Edition itself is licensed separately under the Apache License 2.0 by
Red Gate Software Ltd; consult the Flyway project for its licensing terms.

goway is licensed under the MIT License. Copyright (c) 2026 Cristian Garcia.
