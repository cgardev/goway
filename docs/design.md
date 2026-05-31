# Design

This document explains how goway is structured and how its behavior maps onto
Flyway's, so that the migration sets and operational expectations of Flyway users
carry over.

## Goals

- Reimplement Flyway's core migration model faithfully enough that existing
  migration files and the schema history table behave identically.
- Keep the importable library free of external dependencies; the application
  brings its own `database/sql` driver and connection pool.
- Support only PostgreSQL and SQLite, the latter through a pure Go driver so no
  `cgo` toolchain is required.

## Module layout

The repository is split into several Go modules to protect the zero dependency
contract of the core library.

- `github.com/cgardev/goway` (root) — the library. Standard library only.
- `cmd/goway` — the command line tool, which needs database drivers.
- `example` — a runnable demonstration using SQLite.
- `integration` — database integration tests using testcontainers and the
  drivers.

Within the root module the code is a single flat package named `goway`. The files
group by responsibility: version parsing, file name parsing, the checksum, the
SQL splitter, placeholder replacement, the dialect abstraction, the schema
history table, the migration resolver, the state machine, and one file per
command.

## Dialect abstraction

Database differences are hidden behind a sealed `Dialect` interface whose methods
are unexported, so callers obtain a dialect only through the `Postgres` and
`SQLite` constructors. The interface produces the dialect specific SQL — the
schema history data definition and manipulation statements, identifier quoting,
existence queries, the search path statement, and the clean statements — and
performs statement splitting. Everything else is shared.

When no dialect is configured, the library detects one from a live connection:
the presence of the `sqlite_version` function identifies SQLite, otherwise a
`version()` result containing "PostgreSQL" identifies PostgreSQL.

## Migration naming

A script name is parsed by stripping the suffix, then the prefix, then splitting
the remainder at the first separator. The defaults match Flyway: prefix `V` for
versioned and `R` for repeatable migrations, separator `__`, and suffix `.sql`.
The first configured suffix that matches wins, and the description has its
underscores replaced by spaces, exactly as the reference behavior records it.

## Versions

A version is a sequence of non negative integer components parsed with arbitrary
precision. Underscores are normalized to dots first. Trailing zero components are
dropped so the numeric form is canonical, which means `1`, `1.0` and `1.0.0`
compare and key as equal. Comparison is element by element, treating a missing
component as zero.

## Checksum

The checksum is a CRC32 (IEEE polynomial) computed exactly as Flyway computes it:
the content is read line by line, the bytes of each line are fed to the digest
without the line terminators, and a UTF-8 byte order marker is stripped from the
first line only. The result is interpreted as a signed 32 bit integer for storage
in the history table. Because terminators are excluded, the checksum is
independent of line endings and of a trailing newline. Checksums are computed on
the raw script content, before placeholder replacement.

## Schema history table

The default table is `flyway_schema_history`, kept compatible with Flyway so a
database already managed by Flyway can be adopted. It is created in the default
schema with the same columns: `installed_rank`, `version`, `description`, `type`,
`script`, `checksum`, `installed_by`, `installed_on`, `execution_time` and
`success`. The installed timestamp is supplied by the database. Rows are read and
written with an explicit, consistent column order, and boolean and timestamp
values are decoded defensively so both the native PostgreSQL types and the
textual SQLite representations are accepted.

## The migrate algorithm

1. Resolve the default schema and acquire the migration lock. On PostgreSQL the
   lock is a session advisory lock held on a dedicated connection; SQLite is a
   single writer and needs none.
2. Create any missing schemas, then create the history table if it does not
   exist, recording an optional schema creation marker and an optional baseline.
3. Read the applied migrations and compute the state of every migration.
4. Validate, ignoring the pending migrations that are about to be applied but
   still failing on checksum mismatches, missing or failed migrations.
5. Apply each pending migration in order. Each migration runs in its own
   transaction: the default schema is set as the search path, the statements are
   executed, and the history row is inserted in the same transaction, so a
   failure rolls back the whole migration and leaves no partial state.

Pending migrations are the versioned migrations that have not been applied,
followed by the repeatable migrations whose checksum changed, in version order
then description order.

## State machine

Each migration is classified by correlating the resolved scripts with the applied
history: pending, success, failed, out of order, outdated (a changed repeatable
script), missing (applied but no longer on disk), future (applied with a version
higher than any resolved migration), above target, ignored, and baseline.
Validation reports checksum and description mismatches, missing and failed
migrations, and, for the standalone command, resolved migrations that have not
been applied.

## SQL statement splitting

Scripts are split on the semicolon delimiter while respecting single quoted
strings (including PostgreSQL escape strings), double quoted identifiers, line
and nested block comments, and PostgreSQL dollar quoted bodies. For SQLite the
splitter also recognizes backtick and square bracket identifiers and tracks
`BEGIN`, `CASE` and `END` so the inner statements of a trigger body are kept
together.

## Known divergences

- Only the latest run of a repeatable migration is reported by `Info`; Flyway
  lists every historical run and marks the superseded ones.
- Placeholder checksums are always computed on the raw content; Flyway computes
  them on the replaced content for repeatable migrations.
- Code based migrations, lifecycle callbacks, and undo migrations are not
  implemented.

## Naming and trademark

The library is named goway. Flyway is a trademark of Red Gate Software Ltd; goway
is an independent clean-room reimplementation that references Flyway's documented
behavior only for identification and comparison. The `flyway_schema_history`
table name and the `${flyway:...}` placeholder names are retained as technical
defaults for interoperability and remain configurable.
