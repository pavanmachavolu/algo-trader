package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"algo-trader/backend/database"
)

type PositionResponse struct {
	ID           int64    `json:"id"`
	StockID      int      `json:"stock_id"`
	Symbol       string   `json:"symbol"`
	Name         string   `json:"name"`
	Quantity     int      `json:"quantity"`
	BuyPrice     float64  `json:"buy_price"`
	BuyAmount    float64  `json:"buy_amount"`
	BuyDate      string   `json:"buy_date"`
	Status       string   `json:"status"`
	SellPrice    *float64 `json:"sell_price,omitempty"`
	SellAmount   *float64 `json:"sell_amount,omitempty"`
	SellDate     *string  `json:"sell_date,omitempty"`
	PnL          *float64 `json:"pnl,omitempty"`
	CurrentPrice *float64 `json:"current_price,omitempty"`
	CurrentValue *float64 `json:"current_value,omitempty"`
	UnrealizedPnL *float64 `json:"unrealized_pnl,omitempty"`
}

type SummaryResponse struct {
	TotalInvested float64 `json:"total_invested"`
	CurrentValue  float64 `json:"current_value"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
	UnrealizedPct float64 `json:"unrealized_pct"`
	RealizedPnL   float64 `json:"realized_pnl"`
	OpenCount     int     `json:"open_count"`
	ClosedCount   int     `json:"closed_count"`
}

type TradeResponse struct {
	ID        int64   `json:"id"`
	StockID   int     `json:"stock_id"`
	Symbol    string  `json:"symbol"`
	Name      string  `json:"name"`
	TradeType string  `json:"trade_type"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
	Amount    float64 `json:"amount"`
	TradeDate string  `json:"trade_date"`
}

func GetPositions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	query := `
		SELECT p.id, p.stock_id, s.symbol, s.name, p.quantity, p.buy_price, p.buy_amount,
		       DATE_FORMAT(p.buy_date, '%Y-%m-%d'), p.status,
		       p.sell_price, p.sell_amount, DATE_FORMAT(p.sell_date, '%Y-%m-%d'), p.pnl,
		       lp.price
		FROM positions p
		JOIN stocks s ON s.id = p.stock_id
		LEFT JOIN latest_prices lp ON lp.stock_id = p.stock_id
	`
	args := []interface{}{}
	if status == "OPEN" || status == "CLOSED" {
		query += " WHERE p.status = ?"
		args = append(args, status)
	}
	query += " ORDER BY p.buy_date DESC, p.id DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		log.Printf("Error querying positions: %v", err)
		http.Error(w, "Failed to fetch positions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var positions []PositionResponse
	for rows.Next() {
		var p PositionResponse
		var sellPrice, sellAmount, pnl sql.NullFloat64
		var sellDate sql.NullString
		var currentPrice sql.NullFloat64

		err := rows.Scan(
			&p.ID, &p.StockID, &p.Symbol, &p.Name, &p.Quantity, &p.BuyPrice, &p.BuyAmount,
			&p.BuyDate, &p.Status,
			&sellPrice, &sellAmount, &sellDate, &pnl,
			&currentPrice,
		)
		if err != nil {
			log.Printf("Error scanning position: %v", err)
			continue
		}

		if sellPrice.Valid {
			p.SellPrice = &sellPrice.Float64
		}
		if sellAmount.Valid {
			p.SellAmount = &sellAmount.Float64
		}
		if sellDate.Valid {
			p.SellDate = &sellDate.String
		}
		if pnl.Valid {
			p.PnL = &pnl.Float64
		}
		if currentPrice.Valid && p.Status == "OPEN" {
			p.CurrentPrice = &currentPrice.Float64
			cv := float64(p.Quantity) * currentPrice.Float64
			p.CurrentValue = &cv
			upnl := cv - p.BuyAmount
			p.UnrealizedPnL = &upnl
		}

		positions = append(positions, p)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(positions)
}

func GetPositionsSummary(w http.ResponseWriter, r *http.Request) {
	var summary SummaryResponse

	database.DB.QueryRow(`
		SELECT COALESCE(SUM(p.buy_amount), 0),
		       COALESCE(SUM(p.quantity * lp.price), 0),
		       COUNT(*)
		FROM positions p
		LEFT JOIN latest_prices lp ON lp.stock_id = p.stock_id
		WHERE p.status = 'OPEN'
	`).Scan(&summary.TotalInvested, &summary.CurrentValue, &summary.OpenCount)

	summary.UnrealizedPnL = summary.CurrentValue - summary.TotalInvested
	if summary.TotalInvested > 0 {
		summary.UnrealizedPct = summary.UnrealizedPnL / summary.TotalInvested * 100
	}

	database.DB.QueryRow(
		"SELECT COALESCE(SUM(pnl), 0), COUNT(*) FROM positions WHERE status = 'CLOSED'",
	).Scan(&summary.RealizedPnL, &summary.ClosedCount)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(summary)
}

func GetTrades(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	rows, err := database.DB.Query(`
		SELECT t.id, t.stock_id, s.symbol, s.name, t.trade_type, t.quantity, t.price, t.amount,
		       DATE_FORMAT(t.trade_date, '%Y-%m-%d')
		FROM trades t
		JOIN stocks s ON s.id = t.stock_id
		ORDER BY t.trade_date DESC, t.id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		log.Printf("Error querying trades: %v", err)
		http.Error(w, "Failed to fetch trades", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var trades []TradeResponse
	for rows.Next() {
		var t TradeResponse
		if err := rows.Scan(&t.ID, &t.StockID, &t.Symbol, &t.Name, &t.TradeType, &t.Quantity, &t.Price, &t.Amount, &t.TradeDate); err != nil {
			log.Printf("Error scanning trade: %v", err)
			continue
		}
		trades = append(trades, t)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(trades)
}
