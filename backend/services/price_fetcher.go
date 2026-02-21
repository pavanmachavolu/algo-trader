package services

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"algo-trader/backend/database"

	kiteconnect "github.com/zerodhatech/gokiteconnect"
	"github.com/joho/godotenv"
)

var (
	PriceFetchIntervalMin = 10
	KiteFetchDelayMs      = 200
)

func FetchAndUpdatePrices() error {
	_ = godotenv.Load(".env")
	_ = godotenv.Load("backend/.env")
	apiKey := os.Getenv("KITE_API_KEY")
	accessToken := os.Getenv("KITE_ACCESS_TOKEN")
	if apiKey == "" || accessToken == "" {
		return fmt.Errorf("KITE_API_KEY and KITE_ACCESS_TOKEN required in .env")
	}

	kc := kiteconnect.New(apiKey)
	kc.SetAccessToken(accessToken)

	rows, err := database.DB.Query("SELECT id, symbol, nse_symbol FROM stocks")
	if err != nil {
		return fmt.Errorf("failed to query stocks: %w", err)
	}
	defer rows.Close()

	var stocks []struct {
		ID        int
		Symbol    string
		NSESymbol string
	}
	for rows.Next() {
		var s struct {
			ID        int
			Symbol    string
			NSESymbol string
		}
		if err := rows.Scan(&s.ID, &s.Symbol, &s.NSESymbol); err != nil {
			continue
		}
		stocks = append(stocks, s)
	}

	// Build Kite instrument keys: NSE:RELIANCE (strip .NS)
	instruments := make([]string, 0, len(stocks))
	symbolToStock := make(map[string]struct {
		ID     int
		Symbol string
	})
	for _, s := range stocks {
		nseSym := strings.TrimSuffix(s.NSESymbol, ".NS")
		key := "NSE:" + nseSym
		instruments = append(instruments, key)
		symbolToStock[key] = struct {
			ID     int
			Symbol string
		}{s.ID, s.Symbol}
	}

	// Get quotes (LTP, OHLC, volume) - up to 500 per call
	quotes, err := kc.GetQuote(instruments...)
	if err != nil {
		return fmt.Errorf("Kite GetQuote: %w", err)
	}

	today := time.Now().Format("2006-01-02")

	for _, key := range instruments {
		q, ok := quotes[key]
		if !ok {
			log.Printf("No quote for %s", key)
			continue
		}
		stock := symbolToStock[key]
		price := q.LastPrice
		prevClose := q.OHLC.Close
		var changePercent float64
		if prevClose > 0 {
			changePercent = ((price - prevClose) / prevClose) * 100
		}
		volume := int64(q.Volume)

		// Update latest_prices
		_, err = database.DB.Exec(`
			INSERT INTO latest_prices (stock_id, price, change_percent, volume)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				price = VALUES(price),
				change_percent = VALUES(change_percent),
				volume = VALUES(volume),
				last_updated = CURRENT_TIMESTAMP
		`, stock.ID, price, changePercent, volume)
		if err != nil {
			log.Printf("Error updating latest_prices for %s: %v", stock.Symbol, err)
			continue
		}

		// Upsert today into stock_prices (use OHLC from quote)
		open, high, low := q.OHLC.Open, q.OHLC.High, q.OHLC.Low
		if open == 0 {
			open = price
		}
		if high == 0 {
			high = price
		}
		if low == 0 {
			low = price
		}
		_, err = database.DB.Exec(`
			INSERT INTO stock_prices (stock_id, date, open, high, low, close, volume)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				open = VALUES(open), high = VALUES(high), low = VALUES(low),
				close = VALUES(close), volume = VALUES(volume)
		`, stock.ID, today, open, high, low, price, volume)
		if err != nil {
			log.Printf("Error upserting stock_prices for %s: %v", stock.Symbol, err)
			continue
		}

		if err := updateDMA20ForStockAndDate(stock.ID, today); err != nil {
			log.Printf("Error updating dma_20 for %s: %v", stock.Symbol, err)
		}

		log.Printf("Updated %s: %.2f (%.2f%%)", stock.Symbol, price, changePercent)
	}

	return nil
}

func updateDMA20ForStockAndDate(stockID int, date string) error {
	rows, err := database.DB.Query(`
		SELECT close FROM stock_prices
		WHERE stock_id = ? AND date <= ?
		ORDER BY date DESC LIMIT 20
	`, stockID, date)
	if err != nil {
		return err
	}
	defer rows.Close()
	var closes []float64
	for rows.Next() {
		var c float64
		if err := rows.Scan(&c); err != nil {
			return err
		}
		closes = append(closes, c)
	}
	if len(closes) < 20 {
		return nil
	}
	sum := 0.0
	for _, c := range closes {
		sum += c
	}
	dma20 := sum / float64(len(closes))
	_, err = database.DB.Exec(`UPDATE stock_prices SET dma_20 = ROUND(?, 2) WHERE stock_id = ? AND date = ?`, dma20, stockID, date)
	return err
}

// SyncHistorical fetches last 30 days of day candles from Kite and populates stock_prices.
// Run on startup or periodically to backfill historical data for 20 DMA.
func SyncHistorical() error {
	_ = godotenv.Load(".env")
	_ = godotenv.Load("backend/.env")
	apiKey := os.Getenv("KITE_API_KEY")
	accessToken := os.Getenv("KITE_ACCESS_TOKEN")
	if apiKey == "" || accessToken == "" {
		return fmt.Errorf("KITE_API_KEY and KITE_ACCESS_TOKEN required")
	}

	kc := kiteconnect.New(apiKey)
	kc.SetAccessToken(accessToken)

	instruments, err := kc.GetInstrumentsByExchange("NSE")
	if err != nil {
		return fmt.Errorf("GetInstruments: %w", err)
	}
	symbolToToken := make(map[string]int)
	for _, inst := range instruments {
		if inst.Exchange == "NSE" && inst.InstrumentType == "EQ" {
			symbolToToken[inst.Tradingsymbol] = inst.InstrumentToken
		}
	}

	rows, err := database.DB.Query("SELECT id, nse_symbol FROM stocks")
	if err != nil {
		return err
	}
	defer rows.Close()

	end := time.Now()
	start := end.AddDate(0, 0, -365) // 1 year for backtesting

	for rows.Next() {
		var stockID int
		var nseSymbol string
		if err := rows.Scan(&stockID, &nseSymbol); err != nil {
			continue
		}
		symbol := strings.TrimSuffix(nseSymbol, ".NS")
		token, ok := symbolToToken[symbol]
		if !ok {
			log.Printf("SyncHistorical: %s not found in NSE EQ", symbol)
			continue
		}

		hist, err := kc.GetHistoricalData(token, "day", start, end, false)
		if err != nil {
			log.Printf("SyncHistorical: %s GetHistoricalData: %v", symbol, err)
			time.Sleep(time.Duration(KiteFetchDelayMs) * time.Millisecond)
			continue
		}

		for _, h := range hist {
			dateStr := h.Date.Format("2006-01-02")
			_, err = database.DB.Exec(`
				INSERT INTO stock_prices (stock_id, date, open, high, low, close, volume)
				VALUES (?, ?, ?, ?, ?, ?, ?)
				ON DUPLICATE KEY UPDATE
					open = VALUES(open), high = VALUES(high), low = VALUES(low),
					close = VALUES(close), volume = VALUES(volume)
			`, stockID, dateStr, h.Open, h.High, h.Low, h.Close, h.Volume)
			if err != nil {
				log.Printf("SyncHistorical: insert %s %s: %v", symbol, dateStr, err)
			}
			if err := updateDMA20ForStockAndDate(stockID, dateStr); err != nil {
				log.Printf("SyncHistorical: dma %s %s: %v", symbol, dateStr, err)
			}
		}
		log.Printf("Synced historical for %s (%d candles)", symbol, len(hist))
		time.Sleep(time.Duration(KiteFetchDelayMs) * time.Millisecond)
	}

	return nil
}

func StartPriceFetcher() {
	ticker := time.NewTicker(time.Duration(PriceFetchIntervalMin) * time.Minute)
	defer ticker.Stop()

	log.Printf("Price fetcher: Zerodha Kite, interval=%dm", PriceFetchIntervalMin)

	// Sync historical on startup (backfill for 20 DMA)
	log.Println("Syncing historical data from Kite...")
	if err := SyncHistorical(); err != nil {
		log.Printf("SyncHistorical: %v", err)
	}

	log.Println("Starting price fetcher...")
	if err := FetchAndUpdatePrices(); err != nil {
		log.Printf("Initial fetch: %v", err)
	}

	for range ticker.C {
		if err := FetchAndUpdatePrices(); err != nil {
			log.Printf("Fetch prices: %v", err)
		}
	}
}
