//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run tools/reset_pass.go <password>")
		fmt.Fprintln(os.Stderr, "Generates a bcrypt hash from the given password for manual DB insertion.")
		os.Exit(1)
	}
	pass := os.Args[1]
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(string(hash))
}
