package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"algo-trader/backend/database"
	"algo-trader/backend/models"

	"github.com/gorilla/mux"
)

func GetStocks(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT s.id, s.symbol, s.name, s.sector, s.nse_symbol, s.created_at, s.updated_at,
		       lp.price, lp.change_percent, lp.volume, lp.last_updated
		FROM stocks s
		LEFT JOIN latest_prices lp ON s.id = lp.stock_id
		ORDER BY s.symbol
	`

	rows, err := database.DB.Query(query)
	if err != nil {
		log.Printf("Error querying stocks: %v", err)
		http.Error(w, "Failed to fetch stocks", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var stocks []models.StockWithPrice
	var latestFetchTime sql.NullTime

	for rows.Next() {
		var stock models.Stock
		var price sql.NullFloat64
		var changePercent sql.NullFloat64
		var volume sql.NullInt64
		var lastUpdated sql.NullTime

		err := rows.Scan(
			&stock.ID, &stock.Symbol, &stock.Name, &stock.Sector, &stock.NSESymbol,
			&stock.CreatedAt, &stock.UpdatedAt,
			&price, &changePercent, &volume, &lastUpdated,
		)
		if err != nil {
			log.Printf("Error scanning stock: %v", err)
			continue
		}

		stockWithPrice := models.StockWithPrice{
			Stock: stock,
		}

		if price.Valid {
			stockWithPrice.LatestPrice = &models.LatestPrice{
				StockID:      stock.ID,
				Price:        price.Float64,
				ChangePercent: changePercent.Float64,
				Volume:       volume.Int64,
				LastUpdated:  lastUpdated.Time,
			}
			
			// Track the most recent fetch time
			if lastUpdated.Valid && (!latestFetchTime.Valid || lastUpdated.Time.After(latestFetchTime.Time)) {
				latestFetchTime = lastUpdated
			}
		}

		stocks = append(stocks, stockWithPrice)
	}

	// Get the most recent fetch time from database if not found in results
	if !latestFetchTime.Valid {
		err := database.DB.QueryRow("SELECT MAX(last_updated) FROM latest_prices").Scan(&latestFetchTime)
		if err != nil {
			log.Printf("Error getting latest fetch time: %v", err)
		}
	}

	// Create response with metadata
	response := map[string]interface{}{
		"stocks": stocks,
		"metadata": map[string]interface{}{
			"total_stocks": len(stocks),
			"last_price_fetch": latestFetchTime.Time,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(response)
}

func GetStockHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]

	// Get stock_id from symbol
	var stockID int
	err := database.DB.QueryRow("SELECT id FROM stocks WHERE symbol = ?", symbol).Scan(&stockID)
	if err != nil {
		http.Error(w, "Stock not found", http.StatusNotFound)
		return
	}

	// Get query parameters for date range
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	query := `
		SELECT id, stock_id, date, open, high, low, close, volume, created_at
		FROM stock_prices
		WHERE stock_id = ?
	`

	args := []interface{}{stockID}

	if startDate != "" {
		query += " AND date >= ?"
		args = append(args, startDate)
	}

	if endDate != "" {
		query += " AND date <= ?"
		args = append(args, endDate)
	}

	query += " ORDER BY date DESC LIMIT 365"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		log.Printf("Error querying stock history: %v", err)
		http.Error(w, "Failed to fetch stock history", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var prices []models.StockPrice

	for rows.Next() {
		var price models.StockPrice
		err := rows.Scan(
			&price.ID, &price.StockID, &price.Date,
			&price.Open, &price.High, &price.Low, &price.Close, &price.Volume,
			&price.CreatedAt,
		)
		if err != nil {
			log.Printf("Error scanning price: %v", err)
			continue
		}
		prices = append(prices, price)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(prices)
}
