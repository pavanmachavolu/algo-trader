package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"algo-trader/backend/database"
)

type YahooFinanceResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				PreviousClose      float64 `json:"previousClose"`
				RegularMarketTime  int64   `json:"regularMarketTime"`
			} `json:"meta"`
			Indicators struct {
				Quote []struct {
					Volume []int64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

// Price fetcher settings (change here instead of env vars)
var (
	// YahooFetchDelaySec is the delay in seconds between each symbol request (avoids 429 rate limit)
	YahooFetchDelaySec = 2
	// PriceFetchIntervalMin is how often to run a full price fetch (minutes)
	PriceFetchIntervalMin = 10
)

func FetchAndUpdatePrices() error {
	// Get all stocks
	rows, err := database.DB.Query("SELECT id, nse_symbol FROM stocks")
	if err != nil {
		return fmt.Errorf("failed to query stocks: %w", err)
	}
	defer rows.Close()

	var stocks []struct {
		ID        int
		NSESymbol string
	}

	for rows.Next() {
		var stock struct {
			ID        int
			NSESymbol string
		}
		if err := rows.Scan(&stock.ID, &stock.NSESymbol); err != nil {
			log.Printf("Error scanning stock: %v", err)
			continue
		}
		stocks = append(stocks, stock)
	}

	// Rate limit: Yahoo Finance returns 429 if we hit too fast. Delay between each symbol.
	delay := time.Duration(YahooFetchDelaySec) * time.Second

	// Fetch price for each stock
	for i, stock := range stocks {
		if i > 0 {
			time.Sleep(delay)
		}
		price, changePercent, volume, err := fetchYahooFinancePrice(stock.NSESymbol)
		if err != nil {
			log.Printf("Error fetching price for %s: %v", stock.NSESymbol, err)
			continue
		}

		// Update latest_prices table
		query := `
			INSERT INTO latest_prices (stock_id, price, change_percent, volume)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				price = VALUES(price),
				change_percent = VALUES(change_percent),
				volume = VALUES(volume),
				last_updated = CURRENT_TIMESTAMP
		`

		_, err = database.DB.Exec(query, stock.ID, price, changePercent, volume)
		if err != nil {
			log.Printf("Error updating price for stock_id %d: %v", stock.ID, err)
			continue
		}

		log.Printf("Updated price for %s: %.2f (%.2f%%)", stock.NSESymbol, price, changePercent)
	}

	return nil
}

func fetchYahooFinancePrice(symbol string) (float64, float64, int64, error) {
	// 1) Yahoo Finance first (free, no API key; symbol e.g. RELIANCE.NS for NSE)
	price, changePercent, volume, err := fetchYahooChart(symbol)
	if err == nil {
		return price, changePercent, volume, nil
	}
	log.Printf("Yahoo Finance failed for %s: %v, trying NSE price service...", symbol, err)

	// 2) Fallback: NSE price service (nsetools)
	priceServiceURL := os.Getenv("PRICE_SERVICE_URL")
	if priceServiceURL == "" {
		priceServiceURL = "http://price-service:5000"
	}
	symbolClean := symbol
	if len(symbol) > 3 && symbol[len(symbol)-3:] == ".NS" {
		symbolClean = symbol[:len(symbol)-3]
	}
	url := fmt.Sprintf("%s/quote?symbol=%s", priceServiceURL, symbolClean)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err2 := client.Get(url)
	if err2 == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			var nseResp struct {
				Price         float64 `json:"price"`
				ChangePercent float64 `json:"change_percent"`
				Volume        int64   `json:"volume"`
			}
			if json.Unmarshal(body, &nseResp) == nil && nseResp.Price > 0 {
				return nseResp.Price, nseResp.ChangePercent, nseResp.Volume, nil
			}
		}
	}
	return 0, 0, 0, fmt.Errorf("Yahoo and NSE both failed for %s: %w", symbol, err)
}

func fetchYahooChart(symbol string) (float64, float64, int64, error) {
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=1d", symbol)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, 0, 0, err
	}
	// Yahoo returns 429 without a browser-like User-Agent; reduce rate-limit triggers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return 0, 0, 0, fmt.Errorf("status 429 (rate limited); increase YahooFetchDelaySec or PriceFetchIntervalMin in price_fetcher.go")
	}
	if resp.StatusCode != http.StatusOK {
		return 0, 0, 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, 0, err
	}
	var yahooResp YahooFinanceResponse
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return 0, 0, 0, err
	}
	if len(yahooResp.Chart.Result) == 0 {
		return 0, 0, 0, fmt.Errorf("no data for %s", symbol)
	}
	result := yahooResp.Chart.Result[0]
	price := result.Meta.RegularMarketPrice
	previousClose := result.Meta.PreviousClose
	var changePercent float64
	if previousClose > 0 {
		changePercent = ((price - previousClose) / previousClose) * 100
	}
	var volume int64
	if len(result.Indicators.Quote) > 0 && len(result.Indicators.Quote[0].Volume) > 0 {
		volume = result.Indicators.Quote[0].Volume[0]
	}
	return price, changePercent, volume, nil
}

func StartPriceFetcher() {
	ticker := time.NewTicker(time.Duration(PriceFetchIntervalMin) * time.Minute)
	defer ticker.Stop()

	log.Printf("Price fetcher: interval=%dm, Yahoo delay=%ds", PriceFetchIntervalMin, YahooFetchDelaySec)
	// Fetch immediately on start
	log.Println("Starting price fetcher service...")
	if err := FetchAndUpdatePrices(); err != nil {
		log.Printf("Error in initial price fetch: %v", err)
	}

	for range ticker.C {
		if err := FetchAndUpdatePrices(); err != nil {
			log.Printf("Error fetching prices: %v", err)
		}
	}
}
