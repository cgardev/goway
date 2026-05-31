# Contributing

Thank you for your interest in improving this project. This document describes
how the repository is organized and how to build and test it.

## Prerequisites

- Go 1.26 or newer.
- Docker, only for the PostgreSQL integration tests. The SQLite integration
  tests and all unit tests run without it.

## Repository layout

The repository is composed of several Go modules so that the core library stays
free of external dependencies.

- The root module `github.com/cgardev/goway` is the library. It depends only on
  the standard library; database access is performed through `database/sql`, and
  the calling application supplies the driver.
- `cmd/goway` is a separate module containing the command line tool. It depends
  on the PostgreSQL driver `github.com/jackc/pgx/v5` and the pure Go SQLite
  driver `modernc.org/sqlite`.
- `example` is a separate module with a runnable demonstration.
- `integration` is a separate module that holds the database integration tests,
  keeping the test drivers and the container library out of the core module.

## Building and testing

Run the unit tests and static checks for the core library:

```sh
go build ./...
go vet ./...
gofmt -l .
go test ./... -count=1
```

Run the integration tests, which start a PostgreSQL container through
testcontainers and open a temporary SQLite database:

```sh
go -C integration test ./... -count=1
```

Build the command line tool and the example:

```sh
go -C cmd/goway build ./...
go -C example run .
```

## Coding standards

- Format all code with `gofmt`.
- Document every exported identifier with a complete sentence that begins with
  the identifier name.
- Keep the core module free of external dependencies.
- Write clear, self-documenting code, and add comments only where the logic is
  not self-evident.
- Use technical, impersonal English in comments and documentation.
