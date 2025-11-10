# one/db

A lightweight SQLite database abstraction for Go with a stdlib-compatible API and generic scanning.

## Features

- **stdlib-compatible API**: Familiar methods like `QueryContext`, `QueryRowContext`, `ExecContext`, `BeginTx`
- **Generic scanning**: Type-safe `Scan[T]` and `ScanAll[T]` functions eliminate boilerplate
- **Iterator pattern**: Uses Go 1.23+ `iter.Seq2[T, error]` for efficient row iteration
- **Hybrid column mapping**: Automatic snake_case conversion or explicit `db` struct tags
- **Flexible SELECT * support**: Works with any column order (in `ScanAll`)
- **NULL handling**: Supports both pointer types (NULL → nil) and zero values
- **Environment-based init**: Database path from `APP_NAME` environment variable

## Installation

```bash
go get github.com/michaldziurowski/one/db@v0.1.0
```

**Requirements:** Go 1.24+

## Quick Start

```go
package main

import (
    "context"
    "log"
    "github.com/michaldziurowski/one/db"
)

type User struct {
    ID       int    `db:"id"`
    UserName string
    Email    string
}

func main() {
    // Initialize database (uses APP_NAME env var for path)
    close, err := db.Init()
    if err != nil {
        log.Fatal(err)
    }
    defer close()

    ctx := context.Background()

    // Query single row
    row := db.QueryRowContext(ctx, "SELECT * FROM users WHERE id = ?", 1)
    user, err := db.Scan[User](row)
    if err != nil {
        log.Fatal(err)
    }

    // Query multiple rows with iterator
    rows, err := db.QueryContext(ctx, "SELECT * FROM users")
    if err != nil {
        log.Fatal(err)
    }
    for user, err := range db.ScanAll[User](rows) {
        if err != nil {
            log.Fatal(err)
        }
        // use user
    }
}
```

## API Overview

### Initialization

```go
close, err := db.Init()
```

Initializes the SQLite database using the `APP_NAME` environment variable to determine the database path. Returns a cleanup function to close the database connection.

### Query Functions

```go
// Query multiple rows
rows, err := db.QueryContext(ctx, query, args...)

// Query single row
row := db.QueryRowContext(ctx, query, args...)

// Execute without returning rows
result, err := db.ExecContext(ctx, query, args...)

// Begin transaction
tx, err := db.BeginTx(ctx, options)
```

### Generic Scanning

```go
// Scan single row into type T
value, err := db.Scan[T](row)

// Scan multiple rows into iterator of type T
for value, err := range db.ScanAll[T](rows) {
    // handle value
}
```

Works with:
- Structs (maps columns to fields)
- Scalar types (int, string, bool, etc.)
- Pointer types for NULL handling

### Column Mapping

Fields are mapped to database columns using:
1. Explicit `db` struct tags: `db:"column_name"`
2. Automatic snake_case conversion: `UserName` → `user_name`

## Examples

See the [full example](example/main.go) for comprehensive demonstrations including:
- Struct scanning with NULL handling
- Scalar value queries
- Transactions
- Various query patterns

## Database Location

The database file path is determined by the `APP_NAME` environment variable:

```bash
APP_NAME=myapp go run .
```

Creates/opens: `myapp.db`

## License

MIT
