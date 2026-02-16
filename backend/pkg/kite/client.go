package kite

import (
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	kiteconnect "github.com/zerodhatech/gokiteconnect"
)

const (
	kiteBaseURL = "https://api.kite.trade"
)

// Client wraps Kite Connect API (session via SDK, HTTP for historical).
type Client struct {
	apiKey      string
	accessToken string
	http        *http.Client
}

// NewClient creates a client with the given API key and access token.
// Use GetAccessToken(apiKey, apiSecret, requestToken) to obtain the token.
func NewClient(apiKey, accessToken string) *Client {
	return &Client{
		apiKey:      apiKey,
		accessToken: accessToken,
		http:        &http.Client{Timeout: 30 * time.Second},
	}
}

// GetAccessToken exchanges request_token (from login redirect) for access_token
// using the Kite SDK (GenerateSession). Call once per day; access token valid until EOD.
func GetAccessToken(apiKey, apiSecret, requestToken string) (string, error) {
	kc := kiteconnect.New(apiKey)
	data, err := kc.GenerateSession(requestToken, apiSecret)
	if err != nil {
		return "", fmt.Errorf("generate session: %w", err)
	}
	return data.AccessToken, nil
}

// DMA20FromHistorical computes 20-period simple moving average from SDK historical candles.
// Uses the last 20 closes; returns 0 if fewer than 20.
func DMA20FromHistorical(h []kiteconnect.HistoricalData) float64 {
	if len(h) < 20 {
		return 0
	}
	start := len(h) - 20
	var sum float64
	for i := start; i < len(h); i++ {
		sum += h[i].Close
	}
	return sum / 20
}

// authHeader returns the Authorization header value for Kite API.
func (c *Client) authHeader() string {
	return "token " + c.apiKey + ":" + c.accessToken
}

// InstrumentRow represents one row from the instruments CSV.
type InstrumentRow struct {
	InstrumentToken string
	ExchangeToken   string
	TradingSymbol   string
	Name            string
	Segment         string
	Exchange        string
	InstrumentType  string
}

// GetInstruments returns NSE equity instruments. The API returns gzipped CSV.
func (c *Client) GetInstruments(exchange string) ([]InstrumentRow, error) {
	u := kiteBaseURL + "/instruments"
	if exchange != "" {
		u += "/" + exchange
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("X-Kite-Version", "3")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("instruments API: %s: %s", resp.Status, string(body))
	}

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}

	csvReader := csv.NewReader(reader)
	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("instruments CSV too short")
	}

	header := rows[0]
	col := func(name string) int {
		for i, h := range header {
			if h == name {
				return i
			}
		}
		return -1
	}

	var out []InstrumentRow
	for _, row := range rows[1:] {
		if len(row) <= 1 {
			continue
		}
		r := InstrumentRow{}
		if i := col("instrument_token"); i >= 0 && i < len(row) {
			r.InstrumentToken = strings.TrimSpace(row[i])
		}
		if i := col("exchange_token"); i >= 0 && i < len(row) {
			r.ExchangeToken = strings.TrimSpace(row[i])
		}
		if i := col("tradingsymbol"); i >= 0 && i < len(row) {
			r.TradingSymbol = strings.TrimSpace(row[i])
		}
		if i := col("name"); i >= 0 && i < len(row) {
			r.Name = strings.TrimSpace(row[i])
		}
		if i := col("segment"); i >= 0 && i < len(row) {
			r.Segment = strings.TrimSpace(row[i])
		}
		if i := col("exchange"); i >= 0 && i < len(row) {
			r.Exchange = strings.TrimSpace(row[i])
		}
		if i := col("instrument_type"); i >= 0 && i < len(row) {
			r.InstrumentType = strings.TrimSpace(row[i])
		}
		out = append(out, r)
	}
	return out, nil
}

// NSEInstrumentTokenMap returns a map of NSE EQ tradingsymbol -> instrument_token.
func (c *Client) NSEInstrumentTokenMap() (map[string]string, error) {
	instruments, err := c.GetInstruments("NSE")
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, r := range instruments {
		if r.Exchange == "NSE" && r.InstrumentType == "EQ" {
			m[r.TradingSymbol] = r.InstrumentToken
		}
	}
	return m, nil
}

// Candle represents one OHLCV candle (timestamp, open, high, low, close, volume).
type Candle struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

// historicalResponse matches Kite historical API JSON.
type historicalResponse struct {
	Status string `json:"status"`
	Data   struct {
		Candles [][]interface{} `json:"candles"`
	} `json:"data"`
}

// GetHistoricalDay fetches day candles for the given instrument and date range.
func (c *Client) GetHistoricalDay(instrumentToken, from, to string) ([]Candle, error) {
	u := fmt.Sprintf("%s/instruments/historical/%s/day?from=%s&to=%s",
		kiteBaseURL, instrumentToken, url.QueryEscape(from), url.QueryEscape(to))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("X-Kite-Version", "3")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("historical API: %s: %s", resp.Status, string(body))
	}

	var out historicalResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Status != "success" {
		return nil, fmt.Errorf("historical API status: %s", out.Status)
	}

	var candles []Candle
	for _, row := range out.Data.Candles {
		if len(row) < 6 {
			continue
		}
		ts, _ := row[0].(string)
		t, _ := time.Parse("2006-01-02T15:04:05-0700", ts)
		if t.IsZero() {
			t, _ = time.Parse("2006-01-02T15:04:05+0530", ts)
		}
		open, _ := toFloat(row[1])
		high, _ := toFloat(row[2])
		low, _ := toFloat(row[3])
		closeVal, _ := toFloat(row[4])
		vol, _ := toInt64(row[5])
		candles = append(candles, Candle{
			Time:   t,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closeVal,
			Volume: vol,
		})
	}
	return candles, nil
}

func toFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	}
	return 0, false
}

func toInt64(v interface{}) (int64, bool) {
	switch x := v.(type) {
	case float64:
		return int64(x), true
	case int:
		return int64(x), true
	case int64:
		return x, true
	case string:
		i, err := strconv.ParseInt(x, 10, 64)
		return i, err == nil
	}
	return 0, false
}

// DMA20 computes the 20-period simple moving average of closing prices.
// Uses the last 20 candles; returns 0 if fewer than 20.
func DMA20(candles []Candle) float64 {
	if len(candles) < 20 {
		return 0
	}
	// Last 20 closes (candles are usually chronological, last = most recent)
	start := len(candles) - 20
	var sum float64
	for i := start; i < len(candles); i++ {
		sum += candles[i].Close
	}
	return sum / 20
}
