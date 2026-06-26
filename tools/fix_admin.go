//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dsn := os.Getenv("XMECO_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/xmeco?sslmode=disable"
	}
	if len(os.Args) > 1 {
		dsn = os.Args[1]
	}

	newPassword := "admin123"
	if len(os.Args) > 2 {
		newPassword = os.Args[2]
	}
	targetUser := "admin"
	if len(os.Args) > 3 {
		targetUser = os.Args[3]
	}

	// Require --force flag as safety confirmation
	force := false
	for _, a := range os.Args {
		if a == "--force" {
			force = true
			break
		}
	}
	if !force {
		fmt.Println("WARNING: This will overwrite the password for user:", targetUser)
		fmt.Println("Usage: go run tools/fix_admin.go [dsn] [new_password] [username] --force")
		fmt.Println("Set XMECO_DATABASE_URL env var or pass DSN as first argument.")
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "DB connect failed:", err)
		os.Exit(1)
	}
	defer pool.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Hash failed:", err)
		os.Exit(1)
	}

	tag, err := pool.Exec(context.Background(), `UPDATE users SET password_hash=$1 WHERE username=$2`, string(hash), targetUser)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Update failed:", err)
		os.Exit(1)
	}
	if tag.RowsAffected() == 0 {
		fmt.Fprintln(os.Stderr, "No user found with username:", targetUser)
		os.Exit(1)
	}

	fmt.Printf("Password reset OK for user '%s' (%d row(s) affected)\n", targetUser, tag.RowsAffected())
}
