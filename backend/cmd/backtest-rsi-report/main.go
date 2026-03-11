package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"math"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

const backtestDBName = "algo_trader_backtest_rsi"

// ─── Data types ──────────────────────────────────────────────────────────────

type Trade struct {
	ID         int64
	StockID    int
	Symbol     string
	TradeType  string
	Quantity   int
	Price      float64
	Amount     float64
	RSI        float64
	DMA20      float64
	ExitReason string
	TradeDate  string
	PNL        float64
	HasPNL     bool
}

type StockSummary struct {
	Symbol      string
	Buys        int
	Sells       int
	RealizedPNL float64
	UnrealPNL   float64
	TotalPNL    float64
	StillOpen   bool
	TotalInvest float64
}

type MonthSummary struct {
	Month       string
	Buys        int
	Sells       int
	BuyAmount   float64
	SellAmount  float64
	RealizedPNL float64
	CumPNL      float64
}

type OpenPos struct {
	Symbol    string
	Quantity  int
	BuyPrice  float64
	BuyAmount float64
	BuyDate   string
	LastClose float64
	Value     float64
	UnrealPNL float64
}

type EquityPoint struct {
	Date   string
	CumPNL float64
	X      float64
	Y      float64
}

type ReportData struct {
	GeneratedAt      string
	SimStart         string
	SimEnd           string
	RSIBuyThreshold  float64
	RSISellThreshold float64
	StopLossPct      float64
	BuyBudget        float64
	TotalTrades      int
	BuyCount         int
	SellCount        int
	RSIExits         int
	StopLossExits    int
	WinRate          float64
	Wins             int
	Losses           int
	OpenCount        int
	TotalBuyAmount   float64
	TotalSellAmount  float64
	RealizedPNL      float64
	UnrealizedPNL    float64
	OpenInvested     float64
	NetPNL           float64
	ReturnPct        float64
	Stocks           []StockSummary
	Months           []MonthSummary
	Trades           []Trade
	OpenPositions    []OpenPos
	EquityPoints     []EquityPoint
	MaxPNL           float64
	MinPNL           float64
}

// ─── Main ────────────────────────────────────────────────────────────────────

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	_ = godotenv.Load(".env")
	_ = godotenv.Load("backend/.env")

	db := connectDB()
	defer db.Close()

	data := buildReport(db)

	outPath := "backtest-rsi-report.html"
	if len(os.Args) > 1 {
		outPath = os.Args[1]
	}

	writeHTML(data, outPath)
	log.Printf("Report written to %s\n", outPath)

	openBrowser(outPath)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func connectDB() *sql.DB {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "3306")
	user := getEnv("DB_USER", "root")
	pass := getEnv("DB_PASSWORD", "rootpassword")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
		user, pass, host, port, backtestDBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Cannot connect: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("Cannot ping %s: %v (run the backtest-rsi first)", backtestDBName, err)
	}
	return db
}

// ─── Build report data ──────────────────────────────────────────────────────

func buildReport(db *sql.DB) ReportData {
	trades := loadTrades(db)
	lastCloses := loadLastCloses(db)

	if len(trades) == 0 {
		log.Fatal("No trades found in backtest_trades. Run the backtest-rsi first.")
	}

	rd := ReportData{
		GeneratedAt:      time.Now().Format("2006-01-02 15:04:05"),
		RSIBuyThreshold:  30.0,
		RSISellThreshold: 70.0,
		StopLossPct:      -8.0,
		BuyBudget:        20000.0,
	}

	rd.SimStart = trades[0].TradeDate
	rd.SimEnd = trades[len(trades)-1].TradeDate

	openPositions := make(map[string]OpenPos)
	stockMap := make(map[string]*StockSummary)
	monthMap := make(map[string]*MonthSummary)
	var equityPoints []EquityPoint
	cumRealizedPNL := 0.0

	for _, t := range trades {
		ss, ok := stockMap[t.Symbol]
		if !ok {
			ss = &StockSummary{Symbol: t.Symbol}
			stockMap[t.Symbol] = ss
		}

		month := t.TradeDate[:7]
		ms, ok := monthMap[month]
		if !ok {
			ms = &MonthSummary{Month: month}
			monthMap[month] = ms
		}

		if t.TradeType == "BUY" {
			rd.BuyCount++
			rd.TotalBuyAmount += t.Amount
			ss.Buys++
			ss.TotalInvest += t.Amount
			ms.Buys++
			ms.BuyAmount += t.Amount

			openPositions[t.Symbol] = OpenPos{
				Symbol:    t.Symbol,
				Quantity:  t.Quantity,
				BuyPrice:  t.Price,
				BuyAmount: t.Amount,
				BuyDate:   t.TradeDate,
			}
		} else {
			rd.SellCount++
			rd.TotalSellAmount += t.Amount
			rd.RealizedPNL += t.PNL
			ss.Sells++
			ss.RealizedPNL += t.PNL
			ms.Sells++
			ms.SellAmount += t.Amount
			ms.RealizedPNL += t.PNL

			if t.PNL > 0 {
				rd.Wins++
			} else {
				rd.Losses++
			}

			if t.ExitReason == "STOP_LOSS" {
				rd.StopLossExits++
			} else {
				rd.RSIExits++
			}

			delete(openPositions, t.Symbol)

			cumRealizedPNL += t.PNL
			equityPoints = append(equityPoints, EquityPoint{
				Date:   t.TradeDate,
				CumPNL: cumRealizedPNL,
			})
		}
	}

	var openList []OpenPos
	for _, pos := range openPositions {
		lastClose := lastCloses[pos.Symbol]
		pos.LastClose = lastClose
		pos.Value = float64(pos.Quantity) * lastClose
		pos.UnrealPNL = pos.Value - pos.BuyAmount
		openList = append(openList, pos)
		rd.UnrealizedPNL += pos.UnrealPNL
		rd.OpenInvested += pos.BuyAmount

		if ss, ok := stockMap[pos.Symbol]; ok {
			ss.UnrealPNL = pos.UnrealPNL
			ss.StillOpen = true
		}
	}
	sort.Slice(openList, func(i, j int) bool { return openList[i].Symbol < openList[j].Symbol })
	rd.OpenPositions = openList
	rd.OpenCount = len(openList)

	rd.TotalTrades = rd.BuyCount + rd.SellCount
	rd.NetPNL = rd.RealizedPNL + rd.UnrealizedPNL
	if rd.TotalBuyAmount > 0 {
		rd.ReturnPct = rd.NetPNL / rd.TotalBuyAmount * 100
	}
	if rd.SellCount > 0 {
		rd.WinRate = float64(rd.Wins) / float64(rd.SellCount) * 100
	}

	var stockList []StockSummary
	for _, ss := range stockMap {
		ss.TotalPNL = ss.RealizedPNL + ss.UnrealPNL
		stockList = append(stockList, *ss)
	}
	sort.Slice(stockList, func(i, j int) bool { return stockList[i].TotalPNL > stockList[j].TotalPNL })
	rd.Stocks = stockList

	var monthList []MonthSummary
	cumPNL := 0.0
	for _, ms := range monthMap {
		monthList = append(monthList, *ms)
	}
	sort.Slice(monthList, func(i, j int) bool { return monthList[i].Month < monthList[j].Month })
	for i := range monthList {
		cumPNL += monthList[i].RealizedPNL
		monthList[i].CumPNL = cumPNL
	}
	rd.Months = monthList

	rd.Trades = trades

	if len(equityPoints) > 0 {
		rd.MaxPNL = equityPoints[0].CumPNL
		rd.MinPNL = equityPoints[0].CumPNL
		for _, ep := range equityPoints {
			if ep.CumPNL > rd.MaxPNL {
				rd.MaxPNL = ep.CumPNL
			}
			if ep.CumPNL < rd.MinPNL {
				rd.MinPNL = ep.CumPNL
			}
		}
		padding := math.Max(math.Abs(rd.MaxPNL-rd.MinPNL)*0.1, 100)
		rd.MinPNL -= padding
		rd.MaxPNL += padding
		pnlRange := rd.MaxPNL - rd.MinPNL
		if pnlRange == 0 {
			pnlRange = 1
		}

		chartW := 900.0
		chartH := 250.0
		n := float64(len(equityPoints))
		for i := range equityPoints {
			equityPoints[i].X = (float64(i) / math.Max(n-1, 1)) * chartW
			equityPoints[i].Y = chartH - ((equityPoints[i].CumPNL-rd.MinPNL)/pnlRange)*chartH
		}
		rd.EquityPoints = equityPoints
	}

	return rd
}

func loadTrades(db *sql.DB) []Trade {
	rows, err := db.Query(`
		SELECT id, stock_id, symbol, trade_type, quantity, price, amount, rsi, dma_20, COALESCE(exit_reason, ''), trade_date, pnl
		FROM backtest_trades ORDER BY trade_date, id
	`)
	if err != nil {
		log.Fatalf("Query trades: %v", err)
	}
	defer rows.Close()

	var trades []Trade
	for rows.Next() {
		var t Trade
		var pnl sql.NullFloat64
		if err := rows.Scan(&t.ID, &t.StockID, &t.Symbol, &t.TradeType, &t.Quantity,
			&t.Price, &t.Amount, &t.RSI, &t.DMA20, &t.ExitReason, &t.TradeDate, &pnl); err != nil {
			continue
		}
		if pnl.Valid {
			t.PNL = pnl.Float64
			t.HasPNL = true
		}
		trades = append(trades, t)
	}
	return trades
}

func loadLastCloses(db *sql.DB) map[string]float64 {
	rows, err := db.Query(`
		SELECT s.symbol, sp.close
		FROM stock_prices sp
		JOIN stocks s ON s.id = sp.stock_id
		WHERE sp.date = (SELECT MAX(date) FROM stock_prices)
	`)
	if err != nil {
		log.Fatalf("Query last closes: %v", err)
	}
	defer rows.Close()

	m := make(map[string]float64)
	for rows.Next() {
		var symbol string
		var close float64
		if err := rows.Scan(&symbol, &close); err != nil {
			continue
		}
		m[symbol] = close
	}
	return m
}

// ─── HTML output ─────────────────────────────────────────────────────────────

func writeHTML(data ReportData, path string) {
	funcMap := template.FuncMap{
		"sub": func(a, b int) int { return b - a },
		"formatINR": func(v float64) string {
			sign := ""
			if v > 0 {
				sign = "+"
			}
			return fmt.Sprintf("%s%.2f", sign, v)
		},
		"formatPct": func(v float64) string {
			sign := ""
			if v > 0 {
				sign = "+"
			}
			return fmt.Sprintf("%s%.1f%%", sign, v)
		},
		"plain": func(v float64) string {
			return fmt.Sprintf("%.2f", v)
		},
		"plainInt": func(v float64) string {
			return fmt.Sprintf("%.0f", v)
		},
		"pnlClass": func(v float64) string {
			if v > 0 {
				return "positive"
			}
			if v < 0 {
				return "negative"
			}
			return "zero"
		},
		"exitClass": func(reason string) string {
			if reason == "STOP_LOSS" {
				return "stop-loss"
			}
			return "rsi-exit"
		},
		"exitLabel": func(reason string) string {
			if reason == "STOP_LOSS" {
				return "STOP LOSS"
			}
			if reason == "RSI_OVERBOUGHT" {
				return "RSI ≥ 70"
			}
			return reason
		},
		"svgPath": func(points []EquityPoint) template.HTML {
			if len(points) == 0 {
				return ""
			}
			var b strings.Builder
			for i, p := range points {
				if i == 0 {
					fmt.Fprintf(&b, "M%.1f,%.1f", p.X, p.Y)
				} else {
					fmt.Fprintf(&b, " L%.1f,%.1f", p.X, p.Y)
				}
			}
			return template.HTML(b.String())
		},
		"svgFillPath": func(points []EquityPoint, chartH float64) template.HTML {
			if len(points) == 0 {
				return ""
			}
			var b strings.Builder
			fmt.Fprintf(&b, "M%.1f,%.1f", points[0].X, chartH)
			for _, p := range points {
				fmt.Fprintf(&b, " L%.1f,%.1f", p.X, p.Y)
			}
			fmt.Fprintf(&b, " L%.1f,%.1f Z", points[len(points)-1].X, chartH)
			return template.HTML(b.String())
		},
		"zeroLineY": func(minPNL, maxPNL float64) float64 {
			r := maxPNL - minPNL
			if r == 0 {
				return 125
			}
			return 250 - ((0-minPNL)/r)*250
		},
	}

	tmpl, err := template.New("report").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		log.Fatalf("Parse template: %v", err)
	}

	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("Create %s: %v", path, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		log.Fatalf("Execute template: %v", err)
	}
}

func openBrowser(path string) {
	abs := path
	if !strings.HasPrefix(path, "/") {
		if wd, err := os.Getwd(); err == nil {
			abs = wd + "/" + path
		}
	}
	url := "file://" + abs

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		log.Printf("Open %s in your browser", url)
		return
	}
	cmd.Start()
}

// ─── HTML Template ───────────────────────────────────────────────────────────

var htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Backtest Report — RSI + DMA Combo Strategy</title>
<style>
  :root {
    --bg: #0b0e11;
    --surface: #141920;
    --surface2: #1a2029;
    --border: #252d38;
    --text: #e0e4ea;
    --text-dim: #8892a0;
    --accent: #6c9eff;
    --green: #34d399;
    --red: #f87171;
    --yellow: #fbbf24;
    --orange: #fb923c;
    --cyan: #22d3ee;
  }
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: var(--bg);
    color: var(--text);
    line-height: 1.5;
    padding: 2rem;
  }
  .container { max-width: 1200px; margin: 0 auto; }
  h1 { font-size: 1.75rem; font-weight: 600; margin-bottom: 0.25rem; }
  .strategy-badge {
    display: inline-block;
    background: rgba(34, 211, 238, 0.15);
    color: var(--cyan);
    padding: 0.2rem 0.75rem;
    border-radius: 6px;
    font-size: 0.8rem;
    font-weight: 600;
    margin-bottom: 0.5rem;
  }
  .subtitle { color: var(--text-dim); font-size: 0.9rem; margin-bottom: 2rem; }
  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: 1rem;
    margin-bottom: 2rem;
  }
  .card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 1.25rem;
  }
  .card .label {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-dim);
    margin-bottom: 0.25rem;
  }
  .card .value { font-size: 1.5rem; font-weight: 700; font-variant-numeric: tabular-nums; }
  .card .sub { font-size: 0.8rem; color: var(--text-dim); margin-top: 0.15rem; }
  .positive { color: var(--green); }
  .negative { color: var(--red); }
  .zero { color: var(--text-dim); }
  .section {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 1.5rem;
    margin-bottom: 1.5rem;
  }
  .section h2 {
    font-size: 1.1rem; font-weight: 600;
    margin-bottom: 1rem; padding-bottom: 0.75rem;
    border-bottom: 1px solid var(--border);
  }
  table { width: 100%; border-collapse: collapse; font-size: 0.85rem; font-variant-numeric: tabular-nums; }
  thead th {
    text-align: left; padding: 0.5rem 0.75rem; color: var(--text-dim);
    font-weight: 500; font-size: 0.75rem; text-transform: uppercase;
    letter-spacing: 0.04em; border-bottom: 1px solid var(--border);
    position: sticky; top: 0; background: var(--surface);
  }
  thead th.right, td.right { text-align: right; }
  tbody tr { border-bottom: 1px solid var(--border); }
  tbody tr:hover { background: var(--surface2); }
  tbody td { padding: 0.5rem 0.75rem; white-space: nowrap; }
  .badge {
    display: inline-block; padding: 0.1rem 0.5rem;
    border-radius: 4px; font-size: 0.75rem; font-weight: 600;
  }
  .badge.buy { background: rgba(108,158,255,0.15); color: var(--accent); }
  .badge.sell { background: rgba(251,191,36,0.15); color: var(--yellow); }
  .badge.open { background: rgba(108,158,255,0.1); color: var(--accent); font-weight: 400; }
  .badge.stop-loss { background: rgba(248,113,113,0.15); color: var(--red); font-weight: 500; font-size: 0.7rem; }
  .badge.rsi-exit { background: rgba(34,211,238,0.15); color: var(--cyan); font-weight: 500; font-size: 0.7rem; }
  .chart-container { margin: 0.5rem 0; }
  svg text { font-family: inherit; }
  .scroll-table { max-height: 500px; overflow-y: auto; }
  .algo-desc {
    display: flex; gap: 2rem; flex-wrap: wrap;
    font-size: 0.85rem; color: var(--text-dim); margin-bottom: 0.5rem;
  }
  .algo-desc span { color: var(--text); font-weight: 500; }
  footer { text-align: center; color: var(--text-dim); font-size: 0.8rem; padding: 2rem 0 1rem; }
</style>
</head>
<body>
<div class="container">
  <div class="strategy-badge">RSI + DMA Combo</div>
  <h1>Backtest Report: RSI(14) + 20 DMA Strategy</h1>
  <div class="subtitle">
    Simulation: {{.SimStart}} to {{.SimEnd}} &mdash; Generated: {{.GeneratedAt}}
  </div>
  <div class="algo-desc">
    <div>BUY: RSI(14) &lt; <span>{{plainInt .RSIBuyThreshold}}</span> AND 20 DMA is rising</div>
    <div>SELL: RSI(14) &ge; <span>{{plainInt .RSISellThreshold}}</span> OR stop-loss at <span>{{formatPct .StopLossPct}}</span></div>
    <div>Budget: <span>Rs {{plain .BuyBudget}}</span> per stock</div>
  </div>

  <div class="cards">
    <div class="card">
      <div class="label">Total Trades</div>
      <div class="value">{{.TotalTrades}}</div>
      <div class="sub">{{.BuyCount}} buys / {{.SellCount}} sells</div>
    </div>
    <div class="card">
      <div class="label">Win Rate</div>
      <div class="value {{pnlClass .WinRate}}">{{printf "%.1f" .WinRate}}%</div>
      <div class="sub">{{.Wins}} wins / {{.Losses}} losses</div>
    </div>
    <div class="card">
      <div class="label">Sell Breakdown</div>
      <div class="value">{{.SellCount}}</div>
      <div class="sub">{{.RSIExits}} RSI exits / {{.StopLossExits}} stop-loss</div>
    </div>
    <div class="card">
      <div class="label">Realized P&amp;L</div>
      <div class="value {{pnlClass .RealizedPNL}}">Rs {{formatINR .RealizedPNL}}</div>
      <div class="sub">from {{.SellCount}} closed trades</div>
    </div>
    <div class="card">
      <div class="label">Unrealized P&amp;L</div>
      <div class="value {{pnlClass .UnrealizedPNL}}">Rs {{formatINR .UnrealizedPNL}}</div>
      <div class="sub">{{.OpenCount}} positions open</div>
    </div>
    <div class="card">
      <div class="label">Net P&amp;L</div>
      <div class="value {{pnlClass .NetPNL}}">Rs {{formatINR .NetPNL}}</div>
      <div class="sub">realized + unrealized</div>
    </div>
    <div class="card">
      <div class="label">Return on Capital</div>
      <div class="value {{pnlClass .ReturnPct}}">{{formatPct .ReturnPct}}</div>
      <div class="sub">on Rs {{plain .TotalBuyAmount}} deployed</div>
    </div>
  </div>

  {{if .EquityPoints}}
  <div class="section">
    <h2>Cumulative Realized P&amp;L</h2>
    <div class="chart-container">
      <svg viewBox="-60 -20 980 300" width="100%" style="max-height:300px;">
        <line x1="0" y1="{{zeroLineY .MinPNL .MaxPNL}}" x2="900" y2="{{zeroLineY .MinPNL .MaxPNL}}"
              stroke="#374151" stroke-width="1" stroke-dasharray="6,4"/>
        <text x="-8" y="{{zeroLineY .MinPNL .MaxPNL}}" fill="#8892a0" font-size="11" text-anchor="end" dominant-baseline="middle">0</text>
        <path d="{{svgFillPath .EquityPoints 250.0}}" fill="rgba(34,211,238,0.08)" />
        <path d="{{svgPath .EquityPoints}}" fill="none" stroke="#22d3ee" stroke-width="2" stroke-linejoin="round"/>
        {{range .EquityPoints}}
        <circle cx="{{printf "%.1f" .X}}" cy="{{printf "%.1f" .Y}}" r="3" fill="#22d3ee" opacity="0.6"/>
        {{end}}
        <text x="900" y="{{printf "%.0f" (index .EquityPoints (len .EquityPoints | sub 1)).Y}}"
              fill="#22d3ee" font-size="12" text-anchor="start" dx="6" dominant-baseline="middle">
          Rs {{formatINR (index .EquityPoints (len .EquityPoints | sub 1)).CumPNL}}
        </text>
      </svg>
    </div>
  </div>
  {{end}}

  <div class="section">
    <h2>Monthly Breakdown</h2>
    <table>
      <thead>
        <tr>
          <th>Month</th>
          <th class="right">Buys</th>
          <th class="right">Sells</th>
          <th class="right">Bought</th>
          <th class="right">Sold</th>
          <th class="right">Realized P&amp;L</th>
          <th class="right">Cumulative</th>
        </tr>
      </thead>
      <tbody>
        {{range .Months}}
        <tr>
          <td>{{.Month}}</td>
          <td class="right">{{.Buys}}</td>
          <td class="right">{{.Sells}}</td>
          <td class="right">{{plain .BuyAmount}}</td>
          <td class="right">{{plain .SellAmount}}</td>
          <td class="right {{pnlClass .RealizedPNL}}">{{formatINR .RealizedPNL}}</td>
          <td class="right {{pnlClass .CumPNL}}">{{formatINR .CumPNL}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>
  </div>

  <div class="section">
    <h2>Per-Stock Breakdown</h2>
    <div class="scroll-table">
    <table>
      <thead>
        <tr>
          <th>Symbol</th>
          <th class="right">Buys</th>
          <th class="right">Sells</th>
          <th class="right">Realized</th>
          <th class="right">Unrealized</th>
          <th class="right">Total P&amp;L</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {{range .Stocks}}
        <tr>
          <td>{{.Symbol}}</td>
          <td class="right">{{.Buys}}</td>
          <td class="right">{{.Sells}}</td>
          <td class="right {{pnlClass .RealizedPNL}}">{{formatINR .RealizedPNL}}</td>
          <td class="right {{pnlClass .UnrealPNL}}">{{formatINR .UnrealPNL}}</td>
          <td class="right {{pnlClass .TotalPNL}}"><strong>{{formatINR .TotalPNL}}</strong></td>
          <td>{{if .StillOpen}}<span class="badge open">OPEN</span>{{end}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>
    </div>
  </div>

  {{if .OpenPositions}}
  <div class="section">
    <h2>Open Positions</h2>
    <table>
      <thead>
        <tr>
          <th>Symbol</th>
          <th class="right">Qty</th>
          <th class="right">Buy Price</th>
          <th class="right">Invested</th>
          <th>Buy Date</th>
          <th class="right">Last Close</th>
          <th class="right">Value</th>
          <th class="right">Unrealized P&amp;L</th>
        </tr>
      </thead>
      <tbody>
        {{range .OpenPositions}}
        <tr>
          <td>{{.Symbol}}</td>
          <td class="right">{{.Quantity}}</td>
          <td class="right">{{plain .BuyPrice}}</td>
          <td class="right">{{plain .BuyAmount}}</td>
          <td>{{.BuyDate}}</td>
          <td class="right">{{plain .LastClose}}</td>
          <td class="right">{{plain .Value}}</td>
          <td class="right {{pnlClass .UnrealPNL}}"><strong>{{formatINR .UnrealPNL}}</strong></td>
        </tr>
        {{end}}
      </tbody>
    </table>
  </div>
  {{end}}

  <div class="section">
    <h2>Trade Log ({{.TotalTrades}} trades)</h2>
    <div class="scroll-table">
    <table>
      <thead>
        <tr>
          <th>Date</th>
          <th>Type</th>
          <th>Symbol</th>
          <th class="right">Qty</th>
          <th class="right">Price</th>
          <th class="right">Amount</th>
          <th class="right">RSI</th>
          <th class="right">20 DMA</th>
          <th>Exit Reason</th>
          <th class="right">P&amp;L</th>
        </tr>
      </thead>
      <tbody>
        {{range .Trades}}
        <tr>
          <td>{{.TradeDate}}</td>
          <td><span class="badge {{if eq .TradeType "BUY"}}buy{{else}}sell{{end}}">{{.TradeType}}</span></td>
          <td>{{.Symbol}}</td>
          <td class="right">{{.Quantity}}</td>
          <td class="right">{{plain .Price}}</td>
          <td class="right">{{plain .Amount}}</td>
          <td class="right">{{printf "%.1f" .RSI}}</td>
          <td class="right">{{plain .DMA20}}</td>
          <td>{{if .ExitReason}}<span class="badge {{exitClass .ExitReason}}">{{exitLabel .ExitReason}}</span>{{else}}&mdash;{{end}}</td>
          <td class="right {{if .HasPNL}}{{pnlClass .PNL}}{{end}}">{{if .HasPNL}}{{formatINR .PNL}}{{else}}&mdash;{{end}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>
    </div>
  </div>

  <footer>
    Algo Trader Backtest &mdash; RSI(14) + 20 DMA Combo Strategy on Nifty 50 &mdash; Report generated {{.GeneratedAt}}
  </footer>
</div>
</body>
</html>
`
