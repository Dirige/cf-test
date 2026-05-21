package main

import (
	"fmt"
	"os"

	"cfst-server/internal/crypto"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: encrypt <plaintext>")
		os.Exit(1)
	}

	plaintext := os.Args[1]
	encrypted, err := crypto.Encrypt(plaintext)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Encrypted: %s\n", encrypted)
}
