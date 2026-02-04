package models

import "time"

type Stock struct {
	ID        int       `json:"id"`
	Symbol    string    `json:"symbol"`
	Name      string    `json:"name"`
	Sector    string    `json:"sector"`
	NSESymbol string    `json:"nse_symbol"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type StockPrice struct {
	ID        int64     `json:"id"`
	StockID   int       `json:"stock_id"`
	Date      time.Time `json:"date"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    int64     `json:"volume"`
	CreatedAt time.Time `json:"created_at"`
}

type LatestPrice struct {
	ID           int       `json:"id"`
	StockID      int       `json:"stock_id"`
	Price        float64   `json:"price"`
	ChangePercent float64  `json:"change_percent"`
	Volume       int64     `json:"volume"`
	LastUpdated  time.Time `json:"last_updated"`
}

type StockWithPrice struct {
	Stock
	LatestPrice *LatestPrice `json:"latest_price,omitempty"`
}
