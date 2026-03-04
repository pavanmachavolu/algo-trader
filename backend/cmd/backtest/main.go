package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	kiteconnect "github.com/zerodhatech/gokiteconnect"

	"algo-trader/backend/pkg/kite"
)

// ─── Configuration ───────────────────────────────────────────────────────────

const (
	backtestDBName   = "algo_trader_backtest"
	buyBudget        = 20000.0
	buyThresholdPct  = -5.0 // buy when price is this % or more below 20 DMA
	sellThresholdPct = 5.0  // sell when price is this % or more above 20 DMA
	kiteFetchDelayMs = 200
	dmaWindow        = 20
)

// ─── NSE Holidays (same as services/price_fetcher.go) ────────────────────────

var nseHolidays = map[string]bool{
	"2025-01-26": true, "2025-03-14": true, "2025-03-25": true,
	"2025-04-18": true, "2025-04-14": true, "2025-04-21": true,
	"2025-04-10": true, "2025-05-01": true, "2025-06-06": true,
	"2025-07-17": true, "2025-08-15": true, "2025-09-04": true,
	"2025-10-02": true, "2025-10-08": true, "2025-10-31": true,
	"2025-11-01": true, "2025-11-12": true, "2025-12-25": true,
	"2026-01-26": true, "2026-03-03": true, "2026-03-26": true, "2026-03-31": true,
	"2026-04-03": true, "2026-04-14": true, "2026-05-01": true, "2026-05-28": true,
	"2026-06-26": true, "2026-09-14": true, "2026-10-02": true, "2026-10-20": true,
	"2026-11-10": true, "2026-11-24": true, "2026-12-25": true,
}

func isTradingDay(t time.Time) bool {
	return t.Weekday() != time.Saturday && t.Weekday() != time.Sunday &&
		!nseHolidays[t.Format("2006-01-02")]
}

// ─── Types ───────────────────────────────────────────────────────────────────

type stockInfo struct {
	ID        int
	Symbol    string
	Name      string
	Sector    string
	NSESymbol string
}

type priceDay struct {
	Date   string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
	DMA20  float64
}

type openPosition struct {
	StockID   int
	Symbol    string
	Quantity  int
	BuyPrice  float64
	BuyAmount float64
	BuyDate   string
}

type tradeRecord struct {
	StockID   int
	Symbol    string
	TradeType string
	Quantity  int
	Price     float64
	Amount    float64
	DMA20     float64
	PctVsDMA  float64
	TradeDate string
	PNL       float64
}

// ─── Nifty 50 seed data (from mysql/init/seed_stocks.sql) ────────────────────

var nifty50Seeds = []stockInfo{
	{Symbol: "ADANIENT", Name: "Adani Enterprises Ltd", Sector: "Diversified", NSESymbol: "ADANIENT.NS"},
	{Symbol: "ADANIPORTS", Name: "Adani Ports and Special Economic Zone Ltd", Sector: "Services", NSESymbol: "ADANIPORTS.NS"},
	{Symbol: "APOLLOHOSP", Name: "Apollo Hospitals Enterprise Ltd", Sector: "Healthcare", NSESymbol: "APOLLOHOSP.NS"},
	{Symbol: "ASIANPAINT", Name: "Asian Paints Ltd", Sector: "Consumer Goods", NSESymbol: "ASIANPAINT.NS"},
	{Symbol: "AXISBANK", Name: "Axis Bank Ltd", Sector: "Financial Services", NSESymbol: "AXISBANK.NS"},
	{Symbol: "BAJAJ-AUTO", Name: "Bajaj Auto Ltd", Sector: "Automobile", NSESymbol: "BAJAJ-AUTO.NS"},
	{Symbol: "BAJFINANCE", Name: "Bajaj Finance Ltd", Sector: "Financial Services", NSESymbol: "BAJFINANCE.NS"},
	{Symbol: "BAJAJFINSV", Name: "Bajaj Finserv Ltd", Sector: "Financial Services", NSESymbol: "BAJAJFINSV.NS"},
	{Symbol: "BEL", Name: "Bharat Electronics Ltd", Sector: "Defence", NSESymbol: "BEL.NS"},
	{Symbol: "BHARTIARTL", Name: "Bharti Airtel Ltd", Sector: "Telecommunications", NSESymbol: "BHARTIARTL.NS"},
	{Symbol: "CIPLA", Name: "Cipla Ltd", Sector: "Pharmaceuticals", NSESymbol: "CIPLA.NS"},
	{Symbol: "COALINDIA", Name: "Coal India Ltd", Sector: "Energy", NSESymbol: "COALINDIA.NS"},
	{Symbol: "DRREDDY", Name: "Dr. Reddy's Laboratories Ltd", Sector: "Pharmaceuticals", NSESymbol: "DRREDDY.NS"},
	{Symbol: "EICHERMOT", Name: "Eicher Motors Ltd", Sector: "Automobile", NSESymbol: "EICHERMOT.NS"},
	{Symbol: "ETERNAL", Name: "Eternal Ltd (Zomato)", Sector: "Technology", NSESymbol: "ETERNAL.NS"},
	{Symbol: "GRASIM", Name: "Grasim Industries Ltd", Sector: "Construction Materials", NSESymbol: "GRASIM.NS"},
	{Symbol: "HCLTECH", Name: "HCL Technologies Ltd", Sector: "Information Technology", NSESymbol: "HCLTECH.NS"},
	{Symbol: "HDFCBANK", Name: "HDFC Bank Ltd", Sector: "Financial Services", NSESymbol: "HDFCBANK.NS"},
	{Symbol: "HDFCLIFE", Name: "HDFC Life Insurance Company Ltd", Sector: "Financial Services", NSESymbol: "HDFCLIFE.NS"},
	{Symbol: "HINDALCO", Name: "Hindalco Industries Ltd", Sector: "Metals", NSESymbol: "HINDALCO.NS"},
	{Symbol: "HINDUNILVR", Name: "Hindustan Unilever Ltd", Sector: "Consumer Goods", NSESymbol: "HINDUNILVR.NS"},
	{Symbol: "ICICIBANK", Name: "ICICI Bank Ltd", Sector: "Financial Services", NSESymbol: "ICICIBANK.NS"},
	{Symbol: "INDIGO", Name: "InterGlobe Aviation Ltd", Sector: "Aviation", NSESymbol: "INDIGO.NS"},
	{Symbol: "INFY", Name: "Infosys Ltd", Sector: "Information Technology", NSESymbol: "INFY.NS"},
	{Symbol: "ITC", Name: "ITC Ltd", Sector: "Consumer Goods", NSESymbol: "ITC.NS"},
	{Symbol: "JIOFIN", Name: "Jio Financial Services Ltd", Sector: "Financial Services", NSESymbol: "JIOFIN.NS"},
	{Symbol: "JSWSTEEL", Name: "JSW Steel Ltd", Sector: "Metals", NSESymbol: "JSWSTEEL.NS"},
	{Symbol: "KOTAKBANK", Name: "Kotak Mahindra Bank Ltd", Sector: "Financial Services", NSESymbol: "KOTAKBANK.NS"},
	{Symbol: "LT", Name: "Larsen & Toubro Ltd", Sector: "Construction", NSESymbol: "LT.NS"},
	{Symbol: "M&M", Name: "Mahindra & Mahindra Ltd", Sector: "Automobile", NSESymbol: "M&M.NS"},
	{Symbol: "MARUTI", Name: "Maruti Suzuki India Ltd", Sector: "Automobile", NSESymbol: "MARUTI.NS"},
	{Symbol: "MAXHEALTH", Name: "Max Healthcare Institute Ltd", Sector: "Healthcare", NSESymbol: "MAXHEALTH.NS"},
	{Symbol: "NESTLEIND", Name: "Nestle India Ltd", Sector: "Consumer Goods", NSESymbol: "NESTLEIND.NS"},
	{Symbol: "NTPC", Name: "NTPC Ltd", Sector: "Power", NSESymbol: "NTPC.NS"},
	{Symbol: "ONGC", Name: "Oil and Natural Gas Corporation Ltd", Sector: "Energy", NSESymbol: "ONGC.NS"},
	{Symbol: "POWERGRID", Name: "Power Grid Corporation of India Ltd", Sector: "Power", NSESymbol: "POWERGRID.NS"},
	{Symbol: "RELIANCE", Name: "Reliance Industries Ltd", Sector: "Energy", NSESymbol: "RELIANCE.NS"},
	{Symbol: "SBILIFE", Name: "SBI Life Insurance Company Ltd", Sector: "Financial Services", NSESymbol: "SBILIFE.NS"},
	{Symbol: "SHRIRAMFIN", Name: "Shriram Finance Ltd", Sector: "Financial Services", NSESymbol: "SHRIRAMFIN.NS"},
	{Symbol: "SBIN", Name: "State Bank of India", Sector: "Financial Services", NSESymbol: "SBIN.NS"},
	{Symbol: "SUNPHARMA", Name: "Sun Pharmaceutical Industries Ltd", Sector: "Pharmaceuticals", NSESymbol: "SUNPHARMA.NS"},
	{Symbol: "TCS", Name: "Tata Consultancy Services Ltd", Sector: "Information Technology", NSESymbol: "TCS.NS"},
	{Symbol: "TATACONSUM", Name: "Tata Consumer Products Ltd", Sector: "Consumer Goods", NSESymbol: "TATACONSUM.NS"},
	{Symbol: "TMPV", Name: "Tata Motors Passenger Vehicles Ltd", Sector: "Automobile", NSESymbol: "TMPV.NS"},
	{Symbol: "TATASTEEL", Name: "Tata Steel Ltd", Sector: "Metals", NSESymbol: "TATASTEEL.NS"},
	{Symbol: "TECHM", Name: "Tech Mahindra Ltd", Sector: "Information Technology", NSESymbol: "TECHM.NS"},
	{Symbol: "TITAN", Name: "Titan Company Ltd", Sector: "Consumer Goods", NSESymbol: "TITAN.NS"},
	{Symbol: "TRENT", Name: "Trent Ltd", Sector: "Retail", NSESymbol: "TRENT.NS"},
	{Symbol: "ULTRACEMCO", Name: "UltraTech Cement Ltd", Sector: "Construction Materials", NSESymbol: "ULTRACEMCO.NS"},
	{Symbol: "WIPRO", Name: "Wipro Ltd", Sector: "Information Technology", NSESymbol: "WIPRO.NS"},
}

// ─── Main ────────────────────────────────────────────────────────────────────

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	_ = godotenv.Load(".env")
	_ = godotenv.Load("backend/.env")

	log.Println("============================================================")
	log.Println("       BACKTEST: 20 DMA Strategy on Nifty 50")
	log.Println("============================================================")

	// Step 0: Setup database
	log.Println("\n=== Step 0: Setting up database ===")
	db := setupDatabase()
	defer db.Close()

	stocks := seedStocks(db)
	log.Printf("Seeded %d stocks\n", len(stocks))

	// Step 1: Fetch historical data (skip if data already exists)
	var priceCount int
	db.QueryRow("SELECT COUNT(*) FROM stock_prices").Scan(&priceCount)

	if priceCount > 0 {
		log.Printf("\n=== Step 1: Skipping fetch -- stock_prices already has %d rows ===", priceCount)
		log.Println("   (TRUNCATE stock_prices to force re-fetch from Kite)")
	} else {
		log.Println("\n=== Step 1: Fetching 1-year historical data from Kite ===")
		fetchHistoricalData(db, stocks)
	}

	// Step 2: Load prices and compute 20 DMA
	log.Println("\n=== Step 2: Computing 20 DMA for all stocks ===")
	priceData := loadAndComputeDMA(db, stocks)

	// Step 3: Simulate trading
	log.Println("\n=== Step 3: Simulating trading algorithm ===")
	simStart := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	simEnd := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	log.Printf("Simulation period: %s to %s\n", simStart, simEnd)

	trades, openPositions := simulateTrading(priceData, stocks, simStart, simEnd)
	writeTrades(db, trades)

	// Step 4: Print report
	printReport(trades, openPositions, priceData, simStart, simEnd)
}

// ─── Database setup ──────────────────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func setupDatabase() *sql.DB {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "3306")
	user := getEnv("DB_USER", "root")
	pass := getEnv("DB_PASSWORD", "rootpassword")

	// Connect without database to create it
	adminDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/?parseTime=true", user, pass, host, port)
	adminDB, err := sql.Open("mysql", adminDSN)
	if err != nil {
		log.Fatalf("Cannot connect to MySQL: %v", err)
	}

	if _, err := adminDB.Exec("CREATE DATABASE IF NOT EXISTS " + backtestDBName); err != nil {
		adminDB.Close()
		log.Fatalf("Cannot create database %s: %v", backtestDBName, err)
	}
	adminDB.Close()

	// Connect to the backtest database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
		user, pass, host, port, backtestDBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Cannot open %s: %v", backtestDBName, err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("Cannot ping %s: %v", backtestDBName, err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	createTables(db)
	log.Printf("Connected to %s\n", backtestDBName)
	return db
}

func createTables(db *sql.DB) {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS stocks (
			id INT AUTO_INCREMENT PRIMARY KEY,
			symbol VARCHAR(50) UNIQUE NOT NULL,
			name VARCHAR(200) NOT NULL,
			sector VARCHAR(100),
			nse_symbol VARCHAR(50) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_symbol (symbol)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS stock_prices (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			stock_id INT NOT NULL,
			date DATE NOT NULL,
			open DECIMAL(15,2) NOT NULL,
			high DECIMAL(15,2) NOT NULL,
			low DECIMAL(15,2) NOT NULL,
			close DECIMAL(15,2) NOT NULL,
			volume BIGINT NOT NULL,
			dma_20 DECIMAL(15,2) NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY unique_stock_date (stock_id, date),
			FOREIGN KEY (stock_id) REFERENCES stocks(id) ON DELETE CASCADE,
			INDEX idx_stock_date (stock_id, date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS backtest_trades (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			stock_id INT NOT NULL,
			symbol VARCHAR(50) NOT NULL,
			trade_type ENUM('BUY','SELL') NOT NULL,
			quantity INT NOT NULL,
			price DECIMAL(15,2) NOT NULL,
			amount DECIMAL(15,2) NOT NULL,
			dma_20 DECIMAL(15,2) NOT NULL,
			pct_vs_dma DECIMAL(10,4) NOT NULL,
			trade_date DATE NOT NULL,
			pnl DECIMAL(15,2) NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (stock_id) REFERENCES stocks(id) ON DELETE CASCADE,
			INDEX idx_bt_date (trade_date),
			INDEX idx_bt_stock (stock_id, trade_date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			log.Fatalf("CREATE TABLE failed: %v\nSQL: %.100s...", err, stmt)
		}
	}

	// Always start with fresh trade results
	db.Exec("TRUNCATE TABLE backtest_trades")
}

func seedStocks(db *sql.DB) []stockInfo {
	for _, s := range nifty50Seeds {
		db.Exec(`INSERT IGNORE INTO stocks (symbol, name, sector, nse_symbol) VALUES (?, ?, ?, ?)`,
			s.Symbol, s.Name, s.Sector, s.NSESymbol)
	}

	rows, err := db.Query("SELECT id, symbol, name, sector, nse_symbol FROM stocks ORDER BY symbol")
	if err != nil {
		log.Fatalf("Query stocks: %v", err)
	}
	defer rows.Close()

	var stocks []stockInfo
	for rows.Next() {
		var s stockInfo
		if err := rows.Scan(&s.ID, &s.Symbol, &s.Name, &s.Sector, &s.NSESymbol); err != nil {
			continue
		}
		stocks = append(stocks, s)
	}
	return stocks
}

// ─── Kite data fetch ─────────────────────────────────────────────────────────

func getKiteClient() *kiteconnect.Client {
	kite.LoadEnv()
	apiKey := os.Getenv("KITE_API_KEY")
	accessToken := os.Getenv("KITE_ACCESS_TOKEN")

	if apiKey == "" {
		log.Fatal("KITE_API_KEY required in .env")
	}
	if accessToken == "" {
		log.Fatal("KITE_ACCESS_TOKEN required in .env (run kite-token CLI to generate)")
	}
	if !kite.ValidateToken(apiKey, accessToken) {
		log.Fatal("KITE_ACCESS_TOKEN is invalid/expired; run kite-token CLI to refresh")
	}

	kc := kiteconnect.New(apiKey)
	kc.SetAccessToken(accessToken)
	return kc
}

func fetchHistoricalData(db *sql.DB, stocks []stockInfo) {
	kc := getKiteClient()

	instruments, err := kc.GetInstrumentsByExchange("NSE")
	if err != nil {
		log.Fatalf("GetInstruments: %v", err)
	}
	symbolToToken := make(map[string]int)
	for _, inst := range instruments {
		if inst.Exchange == "NSE" && inst.InstrumentType == "EQ" {
			symbolToToken[inst.Tradingsymbol] = inst.InstrumentToken
		}
	}

	// Fetch from 13 months ago (extra month for DMA warm-up) to yesterday
	now := time.Now()
	end := now.AddDate(0, 0, -1)
	start := now.AddDate(-1, -1, 0)

	log.Printf("Fetching Kite historical data: %s to %s\n", start.Format("2006-01-02"), end.Format("2006-01-02"))

	fetched, skipped := 0, 0
	for i, s := range stocks {
		symbol := strings.TrimSuffix(s.NSESymbol, ".NS")
		token, ok := symbolToToken[symbol]
		if !ok {
			log.Printf("[%d/%d] %s: not found in NSE instruments, skipping", i+1, len(stocks), symbol)
			skipped++
			time.Sleep(time.Duration(kiteFetchDelayMs) * time.Millisecond)
			continue
		}

		hist, err := kc.GetHistoricalData(token, "day", start, end, false)
		if err != nil {
			log.Printf("[%d/%d] %s: fetch error: %v", i+1, len(stocks), symbol, err)
			skipped++
			time.Sleep(time.Duration(kiteFetchDelayMs) * time.Millisecond)
			continue
		}

		for _, h := range hist {
			dateStr := h.Date.Format("2006-01-02")
			_, err := db.Exec(`
				INSERT INTO stock_prices (stock_id, date, open, high, low, close, volume)
				VALUES (?, ?, ?, ?, ?, ?, ?)
				ON DUPLICATE KEY UPDATE
					open=VALUES(open), high=VALUES(high), low=VALUES(low),
					close=VALUES(close), volume=VALUES(volume)
			`, s.ID, dateStr, h.Open, h.High, h.Low, h.Close, int64(h.Volume))
			if err != nil {
				log.Printf("  %s %s: insert error: %v", symbol, dateStr, err)
			}
		}

		fetched++
		log.Printf("[%d/%d] %s: %d candles", i+1, len(stocks), symbol, len(hist))
		time.Sleep(time.Duration(kiteFetchDelayMs) * time.Millisecond)
	}

	log.Printf("Fetch complete: %d stocks fetched, %d skipped\n", fetched, skipped)
}

// ─── DMA computation ─────────────────────────────────────────────────────────

func loadAndComputeDMA(db *sql.DB, stocks []stockInfo) map[int][]priceDay {
	priceData := make(map[int][]priceDay)

	for _, s := range stocks {
		rows, err := db.Query(`
			SELECT date, open, high, low, close, volume
			FROM stock_prices WHERE stock_id = ?
			ORDER BY date ASC
		`, s.ID)
		if err != nil {
			log.Printf("%s: query error: %v", s.Symbol, err)
			continue
		}

		var prices []priceDay
		for rows.Next() {
			var p priceDay
			var d time.Time
			if err := rows.Scan(&d, &p.Open, &p.High, &p.Low, &p.Close, &p.Volume); err != nil {
				continue
			}
			p.Date = d.Format("2006-01-02")
			prices = append(prices, p)
		}
		rows.Close()

		if len(prices) == 0 {
			continue
		}

		// Compute DMA20 in memory
		for i := range prices {
			if i < dmaWindow-1 {
				continue
			}
			sum := 0.0
			for j := i - dmaWindow + 1; j <= i; j++ {
				sum += prices[j].Close
			}
			prices[i].DMA20 = math.Round(sum/float64(dmaWindow)*100) / 100
		}

		// Batch-update DMA in DB
		tx, err := db.Begin()
		if err == nil {
			stmt, _ := tx.Prepare("UPDATE stock_prices SET dma_20 = ? WHERE stock_id = ? AND date = ?")
			for _, p := range prices {
				if p.DMA20 > 0 {
					stmt.Exec(p.DMA20, s.ID, p.Date)
				}
			}
			stmt.Close()
			tx.Commit()
		}

		priceData[s.ID] = prices
	}

	total := 0
	for _, p := range priceData {
		total += len(p)
	}
	log.Printf("Loaded %d price rows for %d stocks, DMA computed\n", total, len(priceData))
	return priceData
}

// ─── Trading simulation ─────────────────────────────────────────────────────

func simulateTrading(priceData map[int][]priceDay, stocks []stockInfo, simStart, simEnd string) ([]tradeRecord, map[int]openPosition) {
	// Build a fast lookup: stockID -> date -> priceDay
	priceLookup := make(map[int]map[string]priceDay)
	for stockID, prices := range priceData {
		m := make(map[string]priceDay, len(prices))
		for _, p := range prices {
			m[p.Date] = p
		}
		priceLookup[stockID] = m
	}

	// Collect all unique dates that fall within the simulation window
	dateSet := make(map[string]bool)
	for _, prices := range priceData {
		for _, p := range prices {
			if p.Date >= simStart && p.Date <= simEnd {
				dateSet[p.Date] = true
			}
		}
	}
	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	openPositions := make(map[int]openPosition)
	var trades []tradeRecord

	for _, date := range dates {
		t, _ := time.Parse("2006-01-02", date)
		if !isTradingDay(t) {
			continue
		}

		for _, s := range stocks {
			prices, ok := priceLookup[s.ID]
			if !ok {
				continue
			}
			p, ok := prices[date]
			if !ok || p.DMA20 <= 0 {
				continue
			}

			pctVsDMA := (p.Close - p.DMA20) / p.DMA20 * 100
			_, hasPosition := openPositions[s.ID]

			// BUY: no open position, price >5% below DMA
			if !hasPosition && pctVsDMA <= buyThresholdPct {
				qty := int(math.Floor(buyBudget / p.Close))
				if qty == 0 {
					continue
				}
				amount := float64(qty) * p.Close

				openPositions[s.ID] = openPosition{
					StockID:   s.ID,
					Symbol:    s.Symbol,
					Quantity:  qty,
					BuyPrice:  p.Close,
					BuyAmount: amount,
					BuyDate:   date,
				}
				trades = append(trades, tradeRecord{
					StockID:   s.ID,
					Symbol:    s.Symbol,
					TradeType: "BUY",
					Quantity:  qty,
					Price:     p.Close,
					Amount:    amount,
					DMA20:     p.DMA20,
					PctVsDMA:  pctVsDMA,
					TradeDate: date,
				})
			}

			// SELL: has open position, price >5% above DMA
			if hasPosition && pctVsDMA >= sellThresholdPct {
				pos := openPositions[s.ID]
				sellAmount := float64(pos.Quantity) * p.Close
				pnl := sellAmount - pos.BuyAmount

				trades = append(trades, tradeRecord{
					StockID:   s.ID,
					Symbol:    s.Symbol,
					TradeType: "SELL",
					Quantity:  pos.Quantity,
					Price:     p.Close,
					Amount:    sellAmount,
					DMA20:     p.DMA20,
					PctVsDMA:  pctVsDMA,
					TradeDate: date,
					PNL:       pnl,
				})
				delete(openPositions, s.ID)
			}
		}
	}

	buyCount, sellCount := 0, 0
	for _, t := range trades {
		if t.TradeType == "BUY" {
			buyCount++
		} else {
			sellCount++
		}
	}
	log.Printf("Simulation complete: %d trades (%d buys, %d sells), %d positions still open\n",
		len(trades), buyCount, sellCount, len(openPositions))

	return trades, openPositions
}

// ─── Persist trades ──────────────────────────────────────────────────────────

func writeTrades(db *sql.DB, trades []tradeRecord) {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("writeTrades: begin tx: %v", err)
		return
	}

	stmt, err := tx.Prepare(`
		INSERT INTO backtest_trades (stock_id, symbol, trade_type, quantity, price, amount, dma_20, pct_vs_dma, trade_date, pnl)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		log.Printf("writeTrades: prepare: %v", err)
		tx.Rollback()
		return
	}
	defer stmt.Close()

	for _, t := range trades {
		var pnl *float64
		if t.TradeType == "SELL" {
			pnl = &t.PNL
		}
		_, err := stmt.Exec(t.StockID, t.Symbol, t.TradeType, t.Quantity, t.Price, t.Amount, t.DMA20, t.PctVsDMA, t.TradeDate, pnl)
		if err != nil {
			log.Printf("writeTrades: insert %s %s: %v", t.Symbol, t.TradeDate, err)
		}
	}

	tx.Commit()
	log.Printf("Wrote %d trades to backtest_trades\n", len(trades))
}

// ─── Report ──────────────────────────────────────────────────────────────────

func printReport(trades []tradeRecord, openPositions map[int]openPosition, priceData map[int][]priceDay, simStart, simEnd string) {
	fmt.Println()
	fmt.Println("================================================================")
	fmt.Println("            BACKTEST REPORT: 20 DMA Strategy")
	fmt.Printf("            Period: %s to %s\n", simStart, simEnd)
	fmt.Println("================================================================")
	fmt.Printf("\n  Algorithm:   BUY when price <= %.0f%% vs 20 DMA\n", buyThresholdPct)
	fmt.Printf("               SELL when price >= +%.0f%% vs 20 DMA\n", sellThresholdPct)
	fmt.Printf("  Budget:      Rs %.0f per stock per buy\n", buyBudget)

	// --- Summary ---
	buyCount, sellCount := 0, 0
	totalBuyAmount := 0.0
	totalSellAmount := 0.0
	totalRealizedPNL := 0.0
	wins, losses := 0, 0

	for _, t := range trades {
		if t.TradeType == "BUY" {
			buyCount++
			totalBuyAmount += t.Amount
		} else {
			sellCount++
			totalSellAmount += t.Amount
			totalRealizedPNL += t.PNL
			if t.PNL > 0 {
				wins++
			} else {
				losses++
			}
		}
	}

	// Unrealized P&L: value open positions at last known close
	totalUnrealizedPNL := 0.0
	totalOpenInvested := 0.0
	for stockID, pos := range openPositions {
		prices, ok := priceData[stockID]
		if !ok || len(prices) == 0 {
			continue
		}
		lastClose := prices[len(prices)-1].Close
		currentValue := float64(pos.Quantity) * lastClose
		totalUnrealizedPNL += currentValue - pos.BuyAmount
		totalOpenInvested += pos.BuyAmount
	}

	netPNL := totalRealizedPNL + totalUnrealizedPNL
	winRate := 0.0
	if sellCount > 0 {
		winRate = float64(wins) / float64(sellCount) * 100
	}

	fmt.Println("\n----------------------------------------------------------------")
	fmt.Println("  SUMMARY")
	fmt.Println("----------------------------------------------------------------")
	fmt.Printf("  Total Trades:         %d  (%d buys, %d sells)\n", buyCount+sellCount, buyCount, sellCount)
	fmt.Printf("  Win Rate:             %.1f%%  (%d wins / %d sells)\n", winRate, wins, sellCount)
	fmt.Printf("  Open Positions:       %d\n", len(openPositions))
	fmt.Println()
	fmt.Printf("  Total Capital Bought: Rs %12.2f\n", totalBuyAmount)
	fmt.Printf("  Total Capital Sold:   Rs %12.2f\n", totalSellAmount)
	fmt.Printf("  Realized P&L:         Rs %+12.2f\n", totalRealizedPNL)
	fmt.Printf("  Unrealized P&L:       Rs %+12.2f  (on Rs %.2f invested)\n", totalUnrealizedPNL, totalOpenInvested)
	fmt.Printf("  Net P&L:              Rs %+12.2f\n", netPNL)

	if totalBuyAmount > 0 {
		returnPct := netPNL / totalBuyAmount * 100
		fmt.Printf("  Return on Capital:    %+.2f%%\n", returnPct)
	}

	// --- Per-stock breakdown ---
	type stockPNL struct {
		Symbol       string
		Buys         int
		Sells        int
		RealizedPNL  float64
		UnrealPNL    float64
		TotalInvest  float64
		StillOpen    bool
	}
	stockMap := make(map[string]*stockPNL)
	for _, t := range trades {
		sp, ok := stockMap[t.Symbol]
		if !ok {
			sp = &stockPNL{Symbol: t.Symbol}
			stockMap[t.Symbol] = sp
		}
		if t.TradeType == "BUY" {
			sp.Buys++
			sp.TotalInvest += t.Amount
		} else {
			sp.Sells++
			sp.RealizedPNL += t.PNL
		}
	}
	for _, pos := range openPositions {
		sp, ok := stockMap[pos.Symbol]
		if !ok {
			continue
		}
		sp.StillOpen = true
		prices, ok := priceData[pos.StockID]
		if ok && len(prices) > 0 {
			lastClose := prices[len(prices)-1].Close
			sp.UnrealPNL = float64(pos.Quantity)*lastClose - pos.BuyAmount
		}
	}

	// Sort by total P&L descending
	var stockList []*stockPNL
	for _, sp := range stockMap {
		stockList = append(stockList, sp)
	}
	sort.Slice(stockList, func(i, j int) bool {
		iPNL := stockList[i].RealizedPNL + stockList[i].UnrealPNL
		jPNL := stockList[j].RealizedPNL + stockList[j].UnrealPNL
		return iPNL > jPNL
	})

	fmt.Println("\n----------------------------------------------------------------")
	fmt.Println("  PER-STOCK BREAKDOWN (sorted by total P&L)")
	fmt.Println("----------------------------------------------------------------")
	fmt.Printf("  %-14s %4s %4s %12s %12s %12s %s\n", "SYMBOL", "BUYS", "SELL", "REALIZED", "UNREALIZED", "TOTAL P&L", "")
	fmt.Println("  " + strings.Repeat("-", 78))

	for _, sp := range stockList {
		totalPNL := sp.RealizedPNL + sp.UnrealPNL
		openMark := ""
		if sp.StillOpen {
			openMark = " [OPEN]"
		}
		fmt.Printf("  %-14s %4d %4d %+12.2f %+12.2f %+12.2f%s\n",
			sp.Symbol, sp.Buys, sp.Sells, sp.RealizedPNL, sp.UnrealPNL, totalPNL, openMark)
	}

	// --- Monthly breakdown ---
	type monthPNL struct {
		Month       string
		Buys        int
		Sells       int
		BuyAmount   float64
		SellAmount  float64
		RealizedPNL float64
	}
	monthMap := make(map[string]*monthPNL)
	for _, t := range trades {
		month := t.TradeDate[:7] // "2025-02"
		mp, ok := monthMap[month]
		if !ok {
			mp = &monthPNL{Month: month}
			monthMap[month] = mp
		}
		if t.TradeType == "BUY" {
			mp.Buys++
			mp.BuyAmount += t.Amount
		} else {
			mp.Sells++
			mp.SellAmount += t.Amount
			mp.RealizedPNL += t.PNL
		}
	}

	var months []*monthPNL
	for _, mp := range monthMap {
		months = append(months, mp)
	}
	sort.Slice(months, func(i, j int) bool {
		return months[i].Month < months[j].Month
	})

	fmt.Println("\n----------------------------------------------------------------")
	fmt.Println("  MONTHLY BREAKDOWN")
	fmt.Println("----------------------------------------------------------------")
	fmt.Printf("  %-10s %4s %4s %14s %14s %14s\n", "MONTH", "BUYS", "SELL", "BOUGHT", "SOLD", "REALIZED P&L")
	fmt.Println("  " + strings.Repeat("-", 70))

	cumPNL := 0.0
	for _, mp := range months {
		cumPNL += mp.RealizedPNL
		fmt.Printf("  %-10s %4d %4d %14.2f %14.2f %+14.2f\n",
			mp.Month, mp.Buys, mp.Sells, mp.BuyAmount, mp.SellAmount, mp.RealizedPNL)
	}
	fmt.Println("  " + strings.Repeat("-", 70))
	fmt.Printf("  %-10s %42s %+14.2f\n", "CUMULATIVE", "", cumPNL)

	// --- Trade log ---
	fmt.Println("\n----------------------------------------------------------------")
	fmt.Println("  TRADE LOG")
	fmt.Println("----------------------------------------------------------------")
	fmt.Printf("  %-12s %-4s %-14s %4s %10s %12s %10s %12s\n",
		"DATE", "TYPE", "SYMBOL", "QTY", "PRICE", "AMOUNT", "vs DMA%", "P&L")
	fmt.Println("  " + strings.Repeat("-", 90))

	for _, t := range trades {
		pnlStr := ""
		if t.TradeType == "SELL" {
			pnlStr = fmt.Sprintf("%+.2f", t.PNL)
		}
		fmt.Printf("  %-12s %-4s %-14s %4d %10.2f %12.2f %+9.1f%% %12s\n",
			t.TradeDate, t.TradeType, t.Symbol, t.Quantity, t.Price, t.Amount, t.PctVsDMA, pnlStr)
	}

	// --- Open positions ---
	if len(openPositions) > 0 {
		fmt.Println("\n----------------------------------------------------------------")
		fmt.Println("  OPEN POSITIONS (not yet sold)")
		fmt.Println("----------------------------------------------------------------")
		fmt.Printf("  %-14s %4s %10s %12s %10s %12s %12s\n",
			"SYMBOL", "QTY", "BUY PRICE", "INVESTED", "LAST PRICE", "VALUE", "UNREAL P&L")
		fmt.Println("  " + strings.Repeat("-", 82))

		// Sort open positions by symbol
		type openPosDisplay struct {
			openPosition
			LastClose float64
		}
		var openList []openPosDisplay
		for stockID, pos := range openPositions {
			opd := openPosDisplay{openPosition: pos}
			if prices, ok := priceData[stockID]; ok && len(prices) > 0 {
				opd.LastClose = prices[len(prices)-1].Close
			}
			openList = append(openList, opd)
		}
		sort.Slice(openList, func(i, j int) bool {
			return openList[i].Symbol < openList[j].Symbol
		})

		for _, pos := range openList {
			value := float64(pos.Quantity) * pos.LastClose
			unrealPNL := value - pos.BuyAmount
			fmt.Printf("  %-14s %4d %10.2f %12.2f %10.2f %12.2f %+12.2f\n",
				pos.Symbol, pos.Quantity, pos.BuyPrice, pos.BuyAmount, pos.LastClose, value, unrealPNL)
		}
	}

	fmt.Println("\n================================================================")
	fmt.Println("                    END OF REPORT")
	fmt.Println("================================================================")
	fmt.Println()
}
