package main

import (
	"context"
	"fmt"
	"log"

	"github.com/michaldziurowski/one/db"
)

type User struct {
	ID          int `db:"id"`
	UserName    string
	Email       string
	OptionalBio *string // nullable field using pointer - NULL becomes nil
	NotWeird    string  // nullable field using string - NULL becomes ""
}

func main() {
	close, err := db.Init()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer close()

	ctx := context.Background()

	fmt.Println("Creating users table...")
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			optional_bio TEXT,
			not_weird TEXT
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	fmt.Println("Inserting sample data...")
	_, err = db.ExecContext(ctx, `
		INSERT OR REPLACE INTO users (id, user_name, email, optional_bio, not_weird) VALUES
		(1, 'John Doe', 'john@example.com', 'Software developer', 'weirdnot'),
		(2, 'Jane Smith', 'jane@example.com', NULL, ''),
		(3, 'Bob Johnson', 'bob@example.com', '', NULL)
	`)
	if err != nil {
		log.Fatalf("Failed to insert data: %v", err)
	}

	// Example 1: Query single row with QueryRowContext + Scan
	fmt.Println("Querying single user by ID:")
	row := db.QueryRowContext(ctx, "SELECT * FROM users WHERE id = ?", 1)
	user, err := db.Scan[User](row)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		bioStr := "<nil>"
		if user.OptionalBio != nil {
			bioStr = *user.OptionalBio
		}
		fmt.Printf("ID: %d, UserName: %s, Email: %s, OptionalBio: '%s', NotWeird: '%s'\n",
			user.ID, user.UserName, user.Email, bioStr, user.NotWeird)
	}

	// Example 2: Query multiple rows with QueryContext + ScanAll
	fmt.Println("\nQuerying users with SELECT * (demonstrates NULL handling):")
	rows, err := db.QueryContext(ctx, "SELECT * FROM users ORDER BY id")
	if err != nil {
		log.Fatalf("Failed to query users: %v", err)
	}
	for user, err := range db.ScanAll[User](rows) {
		if err != nil {
			log.Printf("Error: %v", err)
			break
		}
		bioStr := "<nil>"
		if user.OptionalBio != nil {
			bioStr = *user.OptionalBio
		}
		fmt.Printf("ID: %d, UserName: %s, Email: %s, OptionalBio: '%s', NotWeird: '%s'\n",
			user.ID, user.UserName, user.Email, bioStr, user.NotWeird)
	}

	// Example 3: Query scalar values - user names
	fmt.Println("\nQuerying scalar values - user names:")
	rows, err = db.QueryContext(ctx, "SELECT user_name FROM users ORDER BY id")
	if err != nil {
		log.Fatalf("Failed to query names: %v", err)
	}
	for name, err := range db.ScanAll[string](rows) {
		if err != nil {
			log.Printf("Error: %v", err)
			break
		}
		fmt.Printf("Name: %s\n", name)
	}

	// Example 4: Query scalar value - user count
	fmt.Println("\nQuerying scalar value - user count:")
	countRow := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users")
	count, err := db.Scan[int](countRow)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Total users: %d\n", count)
	}

	// Example 5: Transaction with BeginTx
	fmt.Println("\nDemonstrating transaction:")
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "INSERT INTO users (user_name, email) VALUES (?, ?)", "Alice Cooper", "alice@example.com")
	if err != nil {
		log.Printf("Failed to insert in transaction: %v", err)
	} else {
		fmt.Println("Inserted Alice in transaction")
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("Failed to commit transaction: %v", err)
	} else {
		fmt.Println("Transaction committed successfully")
	}
}
