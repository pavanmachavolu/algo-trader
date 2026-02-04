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

	// Fetch price for each stock
	for _, stock := range stocks {
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
	// Try NSE price service first (more reliable for NSE stocks)
	priceServiceURL := os.Getenv("PRICE_SERVICE_URL")
	if priceServiceURL == "" {
		priceServiceURL = "http://price-service:5000"
	}

	// Try NSE service first - remove .NS suffix for nsetools
	symbolClean := symbol
	if len(symbol) > 3 && symbol[len(symbol)-3:] == ".NS" {
		symbolClean = symbol[:len(symbol)-3]
	}
	
	url := fmt.Sprintf("%s/quote?symbol=%s", priceServiceURL, symbolClean)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				var nseResp struct {
					Price         float64 `json:"price"`
					ChangePercent float64 `json:"change_percent"`
					Volume        int64   `json:"volume"`
				}
				if json.Unmarshal(body, &nseResp) == nil && nseResp.Price > 0 {
					log.Printf("Fetched from NSE service: %s = %.2f", symbol, nseResp.Price)
					return nseResp.Price, nseResp.ChangePercent, nseResp.Volume, nil
				}
			}
		}
	}
	
	// Log that we're falling back to Yahoo Finance
	if err != nil {
		log.Printf("NSE service error for %s: %v, trying Yahoo Finance...", symbol, err)
	}

	// Fallback to Yahoo Finance
	url = fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=1d", symbol)
	client = &http.Client{Timeout: 10 * time.Second}
	resp, err = client.Get(url)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to fetch from price service and Yahoo Finance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, 0, fmt.Errorf("Yahoo Finance API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to read response body: %w", err)
	}

	var yahooResp YahooFinanceResponse
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(yahooResp.Chart.Result) == 0 {
		return 0, 0, 0, fmt.Errorf("no data returned for symbol %s", symbol)
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
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Fetch immediately on start
	log.Println("Starting price fetcher service...")
	if err := FetchAndUpdatePrices(); err != nil {
		log.Printf("Error in initial price fetch: %v", err)
	}

	// Then fetch every 5 minutes
	for range ticker.C {
		if err := FetchAndUpdatePrices(); err != nil {
			log.Printf("Error fetching prices: %v", err)
		}
	}
}
