# Changelog

All notable changes to the one/db package will be documented in this file.

## [Unreleased] - 2025-11-09

### Breaking Changes

**API Refactored to Match stdlib database/sql**

The package has been refactored to align with the standard library's `database/sql` API, providing better compatibility and a more familiar interface for Go developers.

#### Removed

- `Query[T any](ctx context.Context, query string, args ...any) iter.Seq2[T, error]` - Removed the all-in-one generic query method

#### Added

- `QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)` - Returns stdlib `*sql.Rows`
- `QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row` - Returns stdlib `*sql.Row`
- `ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)` - Executes queries without returning rows
- `BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)` - Starts a transaction
- `Scan[T any](row *sql.Row) (T, error)` - Maps a single row to type T
- `ScanAll[T any](rows *sql.Rows) iter.Seq2[T, error]` - Maps multiple rows to an iterator of T

### Migration Guide

**Before (old API):**
```go
// Multiple rows
for user, err := range db.Query[User](ctx, "SELECT * FROM users") {
    if err != nil {
        // handle error
    }
    // use user
}

// Single value
for count, err := range db.Query[int](ctx, "SELECT COUNT(*) FROM users") {
    if err != nil {
        // handle error
    }
    // use count
}
```

**After (new API):**
```go
// Multiple rows
rows, err := db.QueryContext(ctx, "SELECT * FROM users")
if err != nil {
    // handle error
}
for user, err := range db.ScanAll[User](rows) {
    if err != nil {
        // handle error
    }
    // use user
}

// Single row
row := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users")
count, err := db.Scan[int](row)
if err != nil {
    // handle error
}
// use count

// Execute
_, err := db.ExecContext(ctx, "INSERT INTO users (...) VALUES (...)")
if err != nil {
    // handle error
}

// Transaction
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    // handle error
}
defer tx.Rollback()
// ... perform operations with tx
tx.Commit()
```

### Key Differences

1. **stdlib naming**: Uses QueryContext, QueryRowContext, ExecContext, BeginTx to match database/sql exactly
2. **Separation of concerns**: Query operations return stdlib types, scanning is separate
3. **Scan vs ScanAll**: `Scan[T]` for single rows scans fields in declaration order (no column name mapping). `ScanAll[T]` preserves flexible column-to-field mapping with db tags and snake_case conversion
4. **Transaction support**: New `BeginTx` method for explicit transaction management
5. **More idiomatic**: Follows familiar `database/sql` patterns and could fulfill the same interface

### Unchanged Features

- Database initialization with `Init()` using APP_NAME environment variable
- NULL handling with pointer fields and zero values for non-pointer primitives
- Support for both struct and scalar type scanning
- Iterator pattern with `iter.Seq2[T, error]` for memory-efficient row processing
- Automatic snake_case field name conversion (in `ScanAll` only)
- Explicit field mapping with `db` struct tags (in `ScanAll` only)
