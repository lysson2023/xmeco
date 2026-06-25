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
	dsn := "postgres://postgres:xmeco123@localhost:5432/xmeco?sslmode=disable"
	if len(os.Args) > 1 {
		dsn = os.Args[1]
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "DB connect failed:", err)
		os.Exit(1)
	}
	defer pool.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Hash failed:", err)
		os.Exit(1)
	}

	_, err = pool.Exec(context.Background(), `UPDATE users SET password_hash=$1 WHERE username='admin'`, string(hash))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Update failed:", err)
		os.Exit(1)
	}

	fmt.Println("admin password reset OK")
}
