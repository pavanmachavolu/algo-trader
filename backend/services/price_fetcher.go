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

// nseHolidays contains NSE market holidays (YYYY-MM-DD). Update when NSE publishes new year.
// Source: https://www.nseindia.com/market-data/holiday-list
var nseHolidays = map[string]bool{
	// 2025
	"2025-01-26": true, // Republic Day
	"2025-03-14": true, // Mahashivratri
	"2025-03-25": true, // Holi
	"2025-04-18": true, // Good Friday
	"2025-04-14": true, // Ambedkar Jayanti
	"2025-04-21": true, // Ram Navami
	"2025-04-10": true, // Mahavir Jayanti
	"2025-05-01": true, // Maharashtra Day
	"2025-06-06": true, // Bakri-Id
	"2025-07-17": true, // Muharram
	"2025-08-15": true, // Independence Day
	"2025-09-04": true, // Ganesh Chaturthi
	"2025-10-02": true, // Gandhi Jayanti
	"2025-10-08": true, // Dussehra
	"2025-10-31": true, // Diwali Laxmi Pujan
	"2025-11-01": true, // Diwali Balipratipada
	"2025-11-12": true, // Guru Nanak Jayanti
	"2025-12-25": true, // Christmas
	// 2026
	"2026-01-26": true, "2026-03-03": true, "2026-03-26": true, "2026-03-31": true,
	"2026-04-03": true, "2026-04-14": true, "2026-05-01": true, "2026-05-28": true,
	"2026-06-26": true, "2026-09-14": true, "2026-10-02": true, "2026-10-20": true,
	"2026-11-10": true, "2026-11-24": true, "2026-12-25": true,
}

// isTradingDay returns true if the given date is a weekday and not an NSE holiday.
func isTradingDay(t time.Time) bool {
	return t.Weekday() != time.Saturday && t.Weekday() != time.Sunday &&
		!nseHolidays[t.Format("2006-01-02")]
}

// getLastTradingDay returns the most recent trading day (excludes weekends and NSE holidays).
// Never returns today; used so historical sync never includes today.
func getLastTradingDay() time.Time {
	t := time.Now().AddDate(0, 0, -1)
	for !isTradingDay(t) {
		t = t.AddDate(0, 0, -1)
	}
	return t
}

// getLast30TradingDays returns the last 30 trading day dates as "2006-01-02" strings,
// walking backward from getLastTradingDay(). Skips weekends and NSE holidays.
func getLast30TradingDays() []string {
	days := make([]string, 0, 30)
	t := getLastTradingDay()
	for len(days) < 30 {
		if isTradingDay(t) {
			days = append(days, t.Format("2006-01-02"))
		}
		t = t.AddDate(0, 0, -1)
	}
	return days
}

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

// SyncHistorical fetches the last 30 trading days (until yesterday) from Kite for stocks
// that have missing data. Stocks with all 30 dates already in stock_prices are skipped.
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

	// Step 1: Get the 30 trading dates we need
	tradingDays := getLast30TradingDays()
	if len(tradingDays) == 0 {
		return fmt.Errorf("no trading days computed")
	}
	log.Printf("SyncHistorical: checking %d trading days (%s to %s)",
		len(tradingDays), tradingDays[len(tradingDays)-1], tradingDays[0])

	// Step 2: Find stocks missing any of these dates
	// Build placeholders for IN clause
	placeholders := make([]string, len(tradingDays))
	dateArgs := make([]interface{}, len(tradingDays))
	for i, d := range tradingDays {
		placeholders[i] = "?"
		dateArgs[i] = d
	}
	inClause := strings.Join(placeholders, ",")

	query := fmt.Sprintf(`
		SELECT s.id, s.nse_symbol
		FROM stocks s
		WHERE s.id NOT IN (
			SELECT sp.stock_id FROM stock_prices sp
			WHERE sp.date IN (%s)
			GROUP BY sp.stock_id
			HAVING COUNT(DISTINCT sp.date) = ?
		)
	`, inClause)
	args := append(dateArgs, len(tradingDays))

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return fmt.Errorf("query missing stocks: %w", err)
	}
	defer rows.Close()

	type stockInfo struct {
		ID        int
		NSESymbol string
	}
	var missingStocks []stockInfo
	for rows.Next() {
		var s stockInfo
		if err := rows.Scan(&s.ID, &s.NSESymbol); err != nil {
			continue
		}
		missingStocks = append(missingStocks, s)
	}

	if len(missingStocks) == 0 {
		log.Println("SyncHistorical: all stocks up to date, nothing to fetch")
		return nil
	}

	// Step 3: Resolve instrument tokens for stocks that need data
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

	// Date range for Kite API: earliest trading day to latest
	end := getLastTradingDay()
	earliest, _ := time.Parse("2006-01-02", tradingDays[len(tradingDays)-1])
	start := earliest.AddDate(0, 0, -1) // one day before earliest to be safe

	// Step 4: Fetch and upsert only for stocks with missing data
	fetched, skipped := 0, 0
	for _, s := range missingStocks {
		symbol := strings.TrimSuffix(s.NSESymbol, ".NS")
		token, ok := symbolToToken[symbol]
		if !ok {
			log.Printf("SyncHistorical: %s not found in NSE EQ", symbol)
			skipped++
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
			`, s.ID, dateStr, h.Open, h.High, h.Low, h.Close, h.Volume)
			if err != nil {
				log.Printf("SyncHistorical: insert %s %s: %v", symbol, dateStr, err)
			}
		}
		fetched++
		log.Printf("SyncHistorical: synced %s (%d candles)", symbol, len(hist))
		time.Sleep(time.Duration(KiteFetchDelayMs) * time.Millisecond)
	}

	log.Printf("SyncHistorical: fetch phase done. fetched=%d, skipped=%d, already_up_to_date=%d",
		fetched, skipped, 50-len(missingStocks))

	// Step 5: Recalculate 20 DMA for all stocks across the 30 trading days
	if err := recalcDMA20ForAllStocks(tradingDays); err != nil {
		log.Printf("SyncHistorical: DMA recalc error: %v", err)
	}

	return nil
}

// recalcDMA20ForAllStocks recomputes 20 DMA for every stock on each of the given dates.
func recalcDMA20ForAllStocks(tradingDays []string) error {
	rows, err := database.DB.Query("SELECT id, symbol FROM stocks")
	if err != nil {
		return fmt.Errorf("query stocks: %w", err)
	}
	defer rows.Close()

	type stock struct {
		ID     int
		Symbol string
	}
	var stocks []stock
	for rows.Next() {
		var s stock
		if err := rows.Scan(&s.ID, &s.Symbol); err != nil {
			continue
		}
		stocks = append(stocks, s)
	}

	log.Printf("SyncHistorical: recalculating 20 DMA for %d stocks x %d dates", len(stocks), len(tradingDays))
	for _, s := range stocks {
		for _, d := range tradingDays {
			if err := updateDMA20ForStockAndDate(s.ID, d); err != nil {
				log.Printf("SyncHistorical: dma %s %s: %v", s.Symbol, d, err)
			}
		}
	}
	log.Println("SyncHistorical: 20 DMA recalculation complete")
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
