// kite-token generates a Kite access token via the interactive browser login flow.
// Set redirect URL to http://localhost:8888 in your Kite app settings.
//
// Usage: go run ./cmd/kite-token
package main

import (
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

	token, err := kite.RunLoginFlow(apiKey, apiSecret)
	if err != nil {
		log.Fatalf("Login flow failed: %v", err)
	}

	log.Println("Access token:", token)
	log.Println("Saved to .env")
}
