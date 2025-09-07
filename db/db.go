// Package db provides a SQLite database abstraction with generic query support.
//
// This package offers a simple interface for SQLite operations using modernc.org/sqlite
// driver. It supports both struct mapping with automatic snake_case conversion and
// scalar value queries through Go generics.
//
// Key features:
//   - Hybrid column mapping: explicit db tags or automatic snake_case conversion
//   - Support for SELECT * queries with any column order
//   - Generic Query[T] function supporting both structs and scalar types
//   - Iterator-based results with iter.Seq2[T, error] for proper error handling
//   - Database initialization from APP_NAME environment variable
//
// Example usage:
//
//	close, err := db.Init()
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer close()
//
//	// Struct queries
//	for user, err := range db.Query[User](ctx, "SELECT * FROM users") {
//		if err != nil {
//			// handle error
//		}
//		// use user
//	}
//
//	// Scalar queries
//	for name, err := range db.Query[string](ctx, "SELECT user_name FROM users") {
//		if err != nil {
//			// handle error
//		}
//		// use name
//	}
package db

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"os"
	"reflect"
	"strings"
	"unicode"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func Init() (func() error, error) {
	appName := os.Getenv("APP_NAME")
	if appName == "" {
		return nil, fmt.Errorf("APP_NAME environment variable is required")
	}

	dbPath := appName + ".db"
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db = conn

	closeFunc := func() error {
		if db != nil {
			err := db.Close()
			db = nil
			return err
		}
		return nil
	}

	return closeFunc, nil
}

func Query[T any](ctx context.Context, query string, args ...any) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T

		if db == nil {
			yield(zero, fmt.Errorf("database not initialized, call Init() first"))
			return
		}

		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			yield(zero, fmt.Errorf("failed to execute query: %w", err))
			return
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			yield(zero, fmt.Errorf("failed to get columns: %w", err))
			return
		}

		for rows.Next() {
			select {
			case <-ctx.Done():
				yield(zero, ctx.Err())
				return
			default:
			}

			result, err := scanRow[T](rows, columns)
			if err != nil {
				yield(zero, fmt.Errorf("failed to scan row: %w", err))
				return
			}

			if !yield(result, nil) {
				return
			}
		}

		if err := rows.Err(); err != nil {
			yield(zero, fmt.Errorf("row iteration error: %w", err))
		}
	}
}

func scanRow[T any](rows *sql.Rows, columns []string) (T, error) {
	var result T
	resultType := reflect.TypeOf(result)

	if resultType.Kind() != reflect.Struct {
		if len(columns) != 1 {
			return result, fmt.Errorf("scalar type %v requires exactly 1 column, got %d", resultType, len(columns))
		}
		if err := rows.Scan(&result); err != nil {
			return result, err
		}
		return result, nil
	}

	resultValue := reflect.ValueOf(&result).Elem()

	scanValues := make([]any, len(columns))
	columnToField := make(map[string]reflect.Value)

	for i := 0; i < resultType.NumField(); i++ {
		field := resultType.Field(i)
		fieldValue := resultValue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		columnName := field.Tag.Get("db")
		if columnName == "" {
			columnName = toSnakeCase(field.Name)
		}

		columnToField[columnName] = fieldValue
	}

	for i, column := range columns {
		if fieldValue, exists := columnToField[column]; exists {
			scanValues[i] = fieldValue.Addr().Interface()
		} else {
			var dummy any
			scanValues[i] = &dummy
		}
	}

	if err := rows.Scan(scanValues...); err != nil {
		return result, err
	}

	return result, nil
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized, call Init() first")
	}
	return db.ExecContext(ctx, query, args...)
}
