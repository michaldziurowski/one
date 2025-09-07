package main

import (
	"context"
	"fmt"
	"log"

	"db"
)

type User struct {
	ID       int `db:"id"`
	UserName string
	Email    string
}

func main() {
	close, err := db.Init()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer close()

	ctx := context.Background()

	fmt.Println("Creating users table...")
	_, err = db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	fmt.Println("Inserting sample data...")
	_, err = db.Exec(ctx, `
		INSERT OR REPLACE INTO users (id, user_name, email) VALUES 
		(1, 'John Doe', 'john@example.com'),
		(2, 'Jane Smith', 'jane@example.com'),
		(3, 'Bob Johnson', 'bob@example.com')
	`)
	if err != nil {
		log.Fatalf("Failed to insert data: %v", err)
	}

	fmt.Println("Querying users with explicit columns:")
	for user, err := range db.Query[User](ctx, "SELECT id, user_name, email FROM users ORDER BY id") {
		if err != nil {
			log.Printf("Error: %v", err)
			break
		}
		fmt.Printf("ID: %d, UserName: %s, Email: %s\n", user.ID, user.UserName, user.Email)
	}

	fmt.Println("\nQuerying users with SELECT * (demonstrates snake_case mapping):")
	for user, err := range db.Query[User](ctx, "SELECT * FROM users WHERE id > ?", 1) {
		if err != nil {
			log.Printf("Error: %v", err)
			break
		}
		fmt.Printf("ID: %d, UserName: %s, Email: %s\n", user.ID, user.UserName, user.Email)
	}

	fmt.Println("\nQuerying scalar values - user names:")
	for name, err := range db.Query[string](ctx, "SELECT user_name FROM users ORDER BY id") {
		if err != nil {
			log.Printf("Error: %v", err)
			break
		}
		fmt.Printf("Name: %s\n", name)
	}

	fmt.Println("\nQuerying scalar values - user count:")
	for count, err := range db.Query[int](ctx, "SELECT COUNT(*) FROM users") {
		if err != nil {
			log.Printf("Error: %v", err)
			break
		}
		fmt.Printf("Total users: %d\n", count)
	}
}

