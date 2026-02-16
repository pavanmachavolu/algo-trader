// Command nifty20dma fetches Nifty 50 stocks from Zerodha Kite, gets historical day candles,
// and computes 20-day moving average (20 DMA). It uses the official gokiteconnect SDK for
// login URL, session (request_token → access_token), instruments, and historical data.
//
// Usage:
//
//	# One-time: get request_token by opening the login URL (printed by the tool), then:
//	export KITE_API_KEY=your_api_key
//	export KITE_API_SECRET=your_api_secret
//	export KITE_REQUEST_TOKEN=request_token_from_redirect_url
//	go run ./cmd/nifty20dma
//
//	# Or if you already have access_token (valid until EOD):
//	export KITE_ACCESS_TOKEN=your_access_token
//	go run ./cmd/nifty20dma
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"algo-trader/backend/pkg/kite"

	kiteconnect "github.com/zerodhatech/gokiteconnect"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env from current dir or backend/ (when run from repo root)
	_ = godotenv.Load(".env")
	_ = godotenv.Load("backend/.env")

	apiKey := os.Getenv("KITE_API_KEY")
	apiSecret := os.Getenv("KITE_API_SECRET")
	requestToken := os.Getenv("KITE_REQUEST_TOKEN")
	accessToken := os.Getenv("KITE_ACCESS_TOKEN")

	if apiKey == "" {
		log.Fatal("KITE_API_KEY is required")
	}

	// Use SDK client for full auth and API flow
	kc := kiteconnect.New(apiKey)

	if accessToken == "" {
		if apiSecret == "" || requestToken == "" {
			fmt.Println("To get access_token, open this URL in your browser (set redirect URL in Kite developer console first):")
			fmt.Println(kc.GetLoginURL())
			fmt.Println("After login, copy the request_token from the redirect URL and run:")
			fmt.Println("  export KITE_REQUEST_TOKEN=<request_token>")
			fmt.Println("  export KITE_API_SECRET=<your_api_secret>")
			log.Fatal("KITE_ACCESS_TOKEN or (KITE_REQUEST_TOKEN + KITE_API_SECRET) required")
		}
		data, err := kc.GenerateSession(requestToken, apiSecret)
		if err != nil {
			log.Fatalf("Failed to get session: %v", err)
		}
		kc.SetAccessToken(data.AccessToken)
		log.Println("Session generated. Use KITE_ACCESS_TOKEN for the rest of the day to avoid re-login.")
	} else {
		kc.SetAccessToken(accessToken)
	}

	// NSE EQ symbols -> instrument token (int) using SDK
	instruments, err := kc.GetInstrumentsByExchange("NSE")
	if err != nil {
		log.Fatalf("Failed to get instruments: %v", err)
	}
	tokenMap := make(map[string]int)
	for _, inst := range instruments {
		if inst.Exchange == "NSE" && inst.InstrumentType == "EQ" {
			tokenMap[inst.Tradingsymbol] = inst.InstrumentToken
		}
	}

	// Date range: last 30 calendar days so we have at least 20 trading days
	end := time.Now()
	start := end.AddDate(0, 0, -30)

	fmt.Println("Symbol\tInstrumentToken\t20 DMA\tLTP\tCandles")
	fmt.Println("------\t---------------\t------\t---\t------")

	for i, symbol := range kite.Nifty50Symbols {
		if i > 0 {
			time.Sleep(200 * time.Millisecond) // avoid rate limits
		}
		token, ok := tokenMap[symbol]
		if !ok {
			fmt.Printf("%s\t(not found in NSE EQ)\n", symbol)
			continue
		}

		hist, err := kc.GetHistoricalData(token, "day", start, end, false)
		if err != nil {
			fmt.Printf("%s\terror: %v\n", symbol, err)
			continue
		}

		dma20 := kite.DMA20FromHistorical(hist)
		ltp := 0.0
		if len(hist) > 0 {
			ltp = hist[len(hist)-1].Close
		}
		fmt.Printf("%s\t%s\t%.2f\t%.2f\t%d\n", symbol, strconv.Itoa(token), dma20, ltp, len(hist))
	}
}
