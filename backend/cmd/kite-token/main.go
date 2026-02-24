// kite-token ensures a valid Kite access token exists in backend/.env.
// If the current token is valid, it exits immediately. Otherwise it opens
// the Kite login URL in the browser and listens on localhost:8888 for the
// callback to obtain a fresh token.
//
// Build:  go build -o kite-token ./cmd/kite-token
// Usage:  ./kite-token
package main

import (
	"fmt"
	"log"
	"os"

	"algo-trader/backend/pkg/kite"
)

func main() {
	kite.LoadEnv()

	apiKey := os.Getenv("KITE_API_KEY")
	apiSecret := os.Getenv("KITE_API_SECRET")
	if apiKey == "" || apiSecret == "" {
		log.Fatal("KITE_API_KEY and KITE_API_SECRET are required in .env")
	}

	token, err := kite.EnsureAccessToken(apiKey, apiSecret)
	if err != nil {
		log.Fatalf("Failed: %v", err)
	}

	fmt.Println()
	fmt.Println("Access token is valid:", token)
	fmt.Println("You can now run: docker-compose up -d")
}
