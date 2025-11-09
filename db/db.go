// Package db provides a SQLite database abstraction compatible with database/sql.
//
// This package offers a simple interface for SQLite operations using modernc.org/sqlite
// driver. It exposes stdlib-compatible methods (QueryContext, QueryRowContext, ExecContext, BeginTx)
// along with generic scanning functions (Scan, ScanAll) for mapping rows to Go types.
//
// Key features:
//   - stdlib-compatible API: QueryContext, QueryRowContext, ExecContext, BeginTx
//   - Generic Scan[T] for single row mapping
//   - Generic ScanAll[T] for multiple rows with iterator pattern
//   - Hybrid column mapping: explicit db tags or automatic snake_case conversion
//   - Support for SELECT * queries with any column order (ScanAll only)
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
//	// Single row query
//	row := db.QueryRowContext(ctx, "SELECT * FROM users WHERE id = ?", 1)
//	user, err := db.Scan[User](row)
//	if err != nil {
//		// handle error
//	}
//
//	// Multiple rows query
//	rows, err := db.QueryContext(ctx, "SELECT * FROM users ORDER BY id")
//	if err != nil {
//		// handle error
//	}
//	for user, err := range db.ScanAll[User](rows) {
//		if err != nil {
//			// handle error
//		}
//		// use user
//	}
//
//	// Transaction
//	tx, err := db.BeginTx(ctx, nil)
//	if err != nil {
//		// handle error
//	}
//	defer tx.Rollback()
//	// ... use tx
//	tx.Commit()
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

// QueryContext executes a query that returns rows, typically a SELECT.
// The args are for any placeholder parameters in the query.
func QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized, call Init() first")
	}
	return db.QueryContext(ctx, query, args...)
}

// QueryRowContext executes a query that is expected to return at most one row.
// The args are for any placeholder parameters in the query.
func QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return db.QueryRowContext(ctx, query, args...)
}

// BeginTx starts a transaction. The default isolation level is dependent on the driver.
func BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized, call Init() first")
	}
	return db.BeginTx(ctx, opts)
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
	nullableFields := make(map[int]reflect.Value) // Track fields that need NULL handling

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
			fieldType := fieldValue.Type()

			// Handle non-pointer types that need NULL support
			switch fieldType.Kind() {
			case reflect.String:
				// Use sql.NullString for string fields to handle NULL
				var nullStr sql.NullString
				scanValues[i] = &nullStr
				nullableFields[i] = fieldValue
			case reflect.Int, reflect.Int64:
				// Use sql.NullInt64 for int fields to handle NULL
				var nullInt sql.NullInt64
				scanValues[i] = &nullInt
				nullableFields[i] = fieldValue
			case reflect.Float64:
				// Use sql.NullFloat64 for float64 fields to handle NULL
				var nullFloat sql.NullFloat64
				scanValues[i] = &nullFloat
				nullableFields[i] = fieldValue
			case reflect.Bool:
				// Use sql.NullBool for bool fields to handle NULL
				var nullBool sql.NullBool
				scanValues[i] = &nullBool
				nullableFields[i] = fieldValue
			default:
				// For pointer types and other types, use direct scanning
				scanValues[i] = fieldValue.Addr().Interface()
			}
		} else {
			var dummy any
			scanValues[i] = &dummy
		}
	}

	if err := rows.Scan(scanValues...); err != nil {
		return result, err
	}

	// Convert NULL values to appropriate zero values for non-pointer fields
	for i, fieldValue := range nullableFields {
		switch v := scanValues[i].(type) {
		case *sql.NullString:
			if v.Valid {
				fieldValue.SetString(v.String)
			} else {
				fieldValue.SetString("") // NULL → empty string
			}
		case *sql.NullInt64:
			if v.Valid {
				fieldValue.SetInt(v.Int64)
			} else {
				fieldValue.SetInt(0) // NULL → 0
			}
		case *sql.NullFloat64:
			if v.Valid {
				fieldValue.SetFloat(v.Float64)
			} else {
				fieldValue.SetFloat(0.0) // NULL → 0.0
			}
		case *sql.NullBool:
			if v.Valid {
				fieldValue.SetBool(v.Bool)
			} else {
				fieldValue.SetBool(false) // NULL → false
			}
		}
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

// ExecContext executes a query without returning any rows.
// The args are for any placeholder parameters in the query.
func ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized, call Init() first")
	}
	return db.ExecContext(ctx, query, args...)
}

// Scan scans a single row into a value of type T.
// For scalar types (string, int, etc.), it scans directly.
// For struct types, it scans fields in declaration order with NULL handling.
// Pointer fields receive nil for NULL values, non-pointer primitives receive zero values.
func Scan[T any](row *sql.Row) (T, error) {
	var result T
	resultType := reflect.TypeOf(result)

	// Handle scalar types
	if resultType.Kind() != reflect.Struct {
		if err := row.Scan(&result); err != nil {
			return result, err
		}
		return result, nil
	}

	// Handle struct types - scan fields in declaration order
	resultValue := reflect.ValueOf(&result).Elem()
	scanValues := make([]any, 0, resultType.NumField())
	nullableFields := make([]struct {
		index int
		value reflect.Value
	}, 0)

	for i := 0; i < resultType.NumField(); i++ {
		fieldValue := resultValue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		fieldType := fieldValue.Type()

		// Handle non-pointer types that need NULL support
		switch fieldType.Kind() {
		case reflect.String:
			var nullStr sql.NullString
			scanValues = append(scanValues, &nullStr)
			nullableFields = append(nullableFields, struct {
				index int
				value reflect.Value
			}{len(scanValues) - 1, fieldValue})
		case reflect.Int, reflect.Int64:
			var nullInt sql.NullInt64
			scanValues = append(scanValues, &nullInt)
			nullableFields = append(nullableFields, struct {
				index int
				value reflect.Value
			}{len(scanValues) - 1, fieldValue})
		case reflect.Float64:
			var nullFloat sql.NullFloat64
			scanValues = append(scanValues, &nullFloat)
			nullableFields = append(nullableFields, struct {
				index int
				value reflect.Value
			}{len(scanValues) - 1, fieldValue})
		case reflect.Bool:
			var nullBool sql.NullBool
			scanValues = append(scanValues, &nullBool)
			nullableFields = append(nullableFields, struct {
				index int
				value reflect.Value
			}{len(scanValues) - 1, fieldValue})
		default:
			// For pointer types and other types, use direct scanning
			scanValues = append(scanValues, fieldValue.Addr().Interface())
		}
	}

	if err := row.Scan(scanValues...); err != nil {
		return result, err
	}

	// Convert NULL values to appropriate zero values for non-pointer fields
	for _, nf := range nullableFields {
		switch v := scanValues[nf.index].(type) {
		case *sql.NullString:
			if v.Valid {
				nf.value.SetString(v.String)
			} else {
				nf.value.SetString("") // NULL → empty string
			}
		case *sql.NullInt64:
			if v.Valid {
				nf.value.SetInt(v.Int64)
			} else {
				nf.value.SetInt(0) // NULL → 0
			}
		case *sql.NullFloat64:
			if v.Valid {
				nf.value.SetFloat(v.Float64)
			} else {
				nf.value.SetFloat(0.0) // NULL → 0.0
			}
		case *sql.NullBool:
			if v.Valid {
				nf.value.SetBool(v.Bool)
			} else {
				nf.value.SetBool(false) // NULL → false
			}
		}
	}

	return result, nil
}

// ScanAll scans multiple rows into an iterator of type T.
// For scalar types, each row must have exactly one column.
// For struct types, it maps columns to fields using db tags or snake_case conversion.
// Supports flexible column ordering and NULL handling like the original Query[T].
func ScanAll[T any](rows *sql.Rows) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			yield(zero, fmt.Errorf("failed to get columns: %w", err))
			return
		}

		for rows.Next() {
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
