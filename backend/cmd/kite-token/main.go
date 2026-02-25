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
	"net/http"
	"os"
	"time"

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

	log.Println("Waiting 2 seconds before reloading backend config...")
	time.Sleep(2 * time.Second)

	resp, err := http.Post("http://localhost:8080/api/reload-config", "application/json", nil)
	if err != nil {
		log.Printf("Could not call reload-config API: %v (is the backend running?)", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Println("Backend config reloaded successfully.")
	} else {
		log.Printf("Backend reload-config returned status %d", resp.StatusCode)
	}
}
