package services

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"algo-trader/backend/database"
	"algo-trader/backend/pkg/kite"
	"algo-trader/backend/pkg/notify"
)

const buyBudget = 10000.0

var istLocation *time.Location

func init() {
	var err error
	istLocation, err = time.LoadLocation("Asia/Kolkata")
	if err != nil {
		istLocation = time.FixedZone("IST", 5*3600+30*60)
	}
}

func nowIST() time.Time {
	return time.Now().In(istLocation)
}

// StartTrader runs the trader scheduler. It ticks every 60 seconds and
// triggers token checks at 15:00 IST and trading between 15:20-15:30 IST.
func StartTrader() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	log.Println("Trader: scheduler started (token check=15:00, trade window=15:20-15:30 IST)")

	tokenCheckedToday := ""

	for range ticker.C {
		now := nowIST()
		today := now.Format("2006-01-02")

		if !IsTradingDay(now) {
			continue
		}

		hour, min := now.Hour(), now.Minute()

		// 15:00 IST -- token validation check (once per day)
		if hour == 15 && min >= 0 && min < 1 && tokenCheckedToday != today {
			tokenCheckedToday = today
			checkTokenAndNotify()
		}

		// 15:20-15:30 IST -- trading window
		if hour == 15 && min >= 20 && min <= 30 {
			runTraderIfNeeded(today)
		}
	}
}

func checkTokenAndNotify() {
	kite.LoadEnv()
	apiKey := os.Getenv("KITE_API_KEY")
	token := os.Getenv("KITE_ACCESS_TOKEN")

	if apiKey == "" || token == "" || !kite.ValidateToken(apiKey, token) {
		msg := "Kite access token is *expired/missing*. Run `./kite-token` before 3:20 PM to enable trading."
		log.Printf("Trader: %s", msg)
		notify.Send(msg)
	} else {
		log.Println("Trader: 15:00 token check -- token is valid")
	}
}

func runTraderIfNeeded(today string) {
	var exists int
	err := database.DB.QueryRow("SELECT 1 FROM trader_runs WHERE run_date = ?", today).Scan(&exists)
	if err == nil {
		return // already ran today
	}
	if err != sql.ErrNoRows {
		log.Printf("Trader: error checking trader_runs: %v", err)
		return
	}

	kite.LoadEnv()
	apiKey := os.Getenv("KITE_API_KEY")
	token := os.Getenv("KITE_ACCESS_TOKEN")
	if apiKey == "" || token == "" || !kite.ValidateToken(apiKey, token) {
		msg := "Trader: cannot trade -- Kite token is invalid"
		log.Println(msg)
		notify.Send(msg)
		return
	}

	log.Println("Trader: fetching latest prices before trading...")
	if err := FetchAndUpdatePrices(); err != nil {
		log.Printf("Trader: price fetch failed, aborting trade run: %v", err)
		notify.Send(fmt.Sprintf("Trader aborted -- price fetch failed: %v", err))
		return
	}

	log.Println("Trader: starting trading run for", today)

	buys := runBuyFlow(today)
	sells := runSellFlow(today)

	_, err = database.DB.Exec(
		"INSERT INTO trader_runs (run_date, buys, sells) VALUES (?, ?, ?)",
		today, buys, sells,
	)
	if err != nil {
		log.Printf("Trader: error recording run: %v", err)
	}

	log.Printf("Trader: run complete -- %d buys, %d sells", buys, sells)
	sendDailySummary(buys, sells)
}

func runBuyFlow(today string) int {
	buyCount := 0

	for _, symbol := range kite.Nifty50Symbols {
		var stockID int
		err := database.DB.QueryRow("SELECT id FROM stocks WHERE symbol = ?", symbol).Scan(&stockID)
		if err != nil {
			continue
		}

		// Step 1: Check if we already hold this stock
		var openCount int
		err = database.DB.QueryRow(
			"SELECT COUNT(*) FROM positions WHERE stock_id = ? AND status = 'OPEN'", stockID,
		).Scan(&openCount)
		if err != nil || openCount > 0 {
			continue
		}

		// Step 2: Get current price and 20 DMA
		var currentPrice float64
		err = database.DB.QueryRow(
			"SELECT price FROM latest_prices WHERE stock_id = ?", stockID,
		).Scan(&currentPrice)
		if err != nil || currentPrice <= 0 {
			continue
		}

		var dma20 sql.NullFloat64
		err = database.DB.QueryRow(
			"SELECT dma_20 FROM stock_prices WHERE stock_id = ? AND dma_20 IS NOT NULL ORDER BY date DESC LIMIT 1",
			stockID,
		).Scan(&dma20)
		if err != nil || !dma20.Valid || dma20.Float64 <= 0 {
			continue
		}

		// Step 3: Check if >5% below DMA
		pctVsDMA := (currentPrice - dma20.Float64) / dma20.Float64 * 100
		if pctVsDMA >= -5 {
			continue
		}

		// Step 4: Compute quantity (whole shares)
		quantity := int(math.Floor(buyBudget / currentPrice))
		if quantity == 0 {
			log.Printf("Trader: BUY skip %s -- price %.2f > budget %.0f", symbol, currentPrice, buyBudget)
			continue
		}
		buyAmount := float64(quantity) * currentPrice

		// Step 5: Insert position and trade
		_, err = database.DB.Exec(
			`INSERT INTO positions (stock_id, quantity, buy_price, buy_amount, buy_date, status)
			 VALUES (?, ?, ?, ?, ?, 'OPEN')`,
			stockID, quantity, currentPrice, buyAmount, today,
		)
		if err != nil {
			log.Printf("Trader: BUY insert position %s: %v", symbol, err)
			continue
		}

		_, err = database.DB.Exec(
			`INSERT INTO trades (stock_id, trade_type, quantity, price, amount, trade_date)
			 VALUES (?, 'BUY', ?, ?, ?, ?)`,
			stockID, quantity, currentPrice, buyAmount, today,
		)
		if err != nil {
			log.Printf("Trader: BUY insert trade %s: %v", symbol, err)
		}

		buyCount++
		msg := fmt.Sprintf("BUY %s | Qty: %d | Price: %.2f | Amount: %.2f | vs 20DMA: %.1f%%",
			symbol, quantity, currentPrice, buyAmount, pctVsDMA)
		log.Printf("Trader: %s", msg)
		notify.Send(msg)
	}

	return buyCount
}

func runSellFlow(today string) int {
	sellCount := 0

	rows, err := database.DB.Query(`
		SELECT p.id, p.stock_id, p.quantity, p.buy_price, p.buy_amount, s.symbol
		FROM positions p
		JOIN stocks s ON s.id = p.stock_id
		WHERE p.status = 'OPEN'
	`)
	if err != nil {
		log.Printf("Trader: sell flow query: %v", err)
		return 0
	}
	defer rows.Close()

	type openPos struct {
		ID        int64
		StockID   int
		Quantity  int
		BuyPrice  float64
		BuyAmount float64
		Symbol    string
	}
	var positions []openPos
	for rows.Next() {
		var p openPos
		if err := rows.Scan(&p.ID, &p.StockID, &p.Quantity, &p.BuyPrice, &p.BuyAmount, &p.Symbol); err != nil {
			continue
		}
		positions = append(positions, p)
	}

	for _, p := range positions {
		var currentPrice float64
		err := database.DB.QueryRow(
			"SELECT price FROM latest_prices WHERE stock_id = ?", p.StockID,
		).Scan(&currentPrice)
		if err != nil || currentPrice <= 0 {
			continue
		}

		var dma20 sql.NullFloat64
		err = database.DB.QueryRow(
			"SELECT dma_20 FROM stock_prices WHERE stock_id = ? AND dma_20 IS NOT NULL ORDER BY date DESC LIMIT 1",
			p.StockID,
		).Scan(&dma20)
		if err != nil || !dma20.Valid || dma20.Float64 <= 0 {
			continue
		}

		pctVsDMA := (currentPrice - dma20.Float64) / dma20.Float64 * 100
		if pctVsDMA <= 5 {
			continue
		}

		sellAmount := float64(p.Quantity) * currentPrice
		pnl := sellAmount - p.BuyAmount

		_, err = database.DB.Exec(
			`UPDATE positions SET status='CLOSED', sell_price=?, sell_amount=?, sell_date=?, pnl=?
			 WHERE id=?`,
			currentPrice, sellAmount, today, pnl, p.ID,
		)
		if err != nil {
			log.Printf("Trader: SELL update position %s: %v", p.Symbol, err)
			continue
		}

		_, err = database.DB.Exec(
			`INSERT INTO trades (stock_id, trade_type, quantity, price, amount, trade_date)
			 VALUES (?, 'SELL', ?, ?, ?, ?)`,
			p.StockID, p.Quantity, currentPrice, sellAmount, today,
		)
		if err != nil {
			log.Printf("Trader: SELL insert trade %s: %v", p.Symbol, err)
		}

		sellCount++
		pnlPct := 0.0
		if p.BuyAmount > 0 {
			pnlPct = pnl / p.BuyAmount * 100
		}
		msg := fmt.Sprintf("SELL %s | Qty: %d | Price: %.2f | P&L: %+.2f (%.1f%%)",
			p.Symbol, p.Quantity, currentPrice, pnl, pnlPct)
		log.Printf("Trader: %s", msg)
		notify.Send(msg)
	}

	return sellCount
}

func sendDailySummary(buys, sells int) {
	var totalInvested, totalCurrentValue sql.NullFloat64
	var realizedPnl sql.NullFloat64

	database.DB.QueryRow(`
		SELECT COALESCE(SUM(buy_amount), 0), 
		       COALESCE(SUM(p.quantity * lp.price), 0)
		FROM positions p
		LEFT JOIN latest_prices lp ON lp.stock_id = p.stock_id
		WHERE p.status = 'OPEN'
	`).Scan(&totalInvested, &totalCurrentValue)

	database.DB.QueryRow(
		"SELECT COALESCE(SUM(pnl), 0) FROM positions WHERE status = 'CLOSED'",
	).Scan(&realizedPnl)

	invested := 0.0
	if totalInvested.Valid {
		invested = totalInvested.Float64
	}
	current := 0.0
	if totalCurrentValue.Valid {
		current = totalCurrentValue.Float64
	}
	unrealized := current - invested
	realized := 0.0
	if realizedPnl.Valid {
		realized = realizedPnl.Float64
	}

	msg := fmt.Sprintf(
		"*Trader run complete*\nBuys: %d | Sells: %d\nInvested: %.0f | Value: %.0f\nUnrealized P&L: %+.0f | Realized P&L: %+.0f",
		buys, sells, invested, current, unrealized, realized,
	)
	notify.Send(msg)
}
