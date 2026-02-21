# NSE Nifty 50 Algorithmic Trading App

A full-stack application for monitoring and backtesting algorithmic trading strategies on NSE Nifty 50 stocks.

## Architecture

- **Backend**: Go REST API with MySQL database
- **Frontend**: React.js with Tailwind CSS
- **Database**: MySQL 8.0 with persistent Docker volumes
- **Data Source**: Zerodha Kite Connect (paid subscription, ₹500/month)
- **Deployment**: Docker Compose

## Features

- View all Nifty 50 stocks with latest prices
- Historical data (1 year) from Zerodha Kite for backtesting
- Auto-refresh prices every 10 minutes
- 20-day moving average (20 DMA) computed from Kite historical data
- Persistent data storage in Docker volumes
- Clean, responsive UI

## Prerequisites

- Docker and Docker Compose installed
- Zerodha Kite Connect paid subscription (₹500/month)
- At least 2GB free disk space for database volume

## Quick Start

1. **Set up Zerodha Kite credentials** in `backend/.env`:
   ```
   KITE_API_KEY=your_api_key
   KITE_API_SECRET=your_api_secret
   KITE_ACCESS_TOKEN=your_access_token
   ```
   Use `go run ./cmd/kite-token` to generate access token (valid until EOD).

2. **Start all services:**
   ```bash
   docker-compose up -d
   ```

3. **Access the application:**
   - Frontend: http://localhost:3000
   - Backend API: http://localhost:8080
   - MySQL: localhost:3306

## Services

### MySQL Database
- Port: 3306
- Database: `algo_trader`
- User: `trader` / Password: `traderpass`
- Root Password: `rootpassword`
- Data persisted in `./docker-volumes/mysql-data`
- Nifty 50 stocks seeded on first run

### Go Backend
- Port: 8080
- Endpoints:
  - `GET /api/stocks` - List all stocks with latest prices and 20 DMA
  - `GET /api/stocks/{symbol}/history` - Get historical data for a stock
  - `GET /health` - Health check
- **Price fetcher:** Fetches prices and historical data from Zerodha Kite. Syncs 1 year historical on startup, then fetches quotes every 10 minutes. Computes 20 DMA from stock_prices.

### React Frontend
- Port: 3000
- Auto-refreshes every 5 minutes

### Zerodha Kite CLI Tools
- **kite-token:** Generates access token via browser login. Runs HTTP server on 8888, opens login URL, captures redirect. Set redirect URL to `http://localhost:8888` in Kite app.
- **nifty20dma:** Fetches historical day candles and prints 20 DMA for Nifty 50 (standalone, no DB).

## API Usage

### Get all stocks
```bash
curl http://localhost:8080/api/stocks
```

### Get stock history
```bash
curl http://localhost:8080/api/stocks/RELIANCE/history
```

### Get stock history with date range
```bash
curl "http://localhost:8080/api/stocks/RELIANCE/history?start_date=2024-01-01&end_date=2024-12-31"
```

## Database Schema

- `stocks`: Stock metadata (symbol, name, sector, nse_symbol)
- `stock_prices`: Historical OHLCV data from Kite
- `latest_prices`: Current prices (updated every 10 min)

## Development

### Backend Development
```bash
cd backend
go mod download
# Ensure backend/.env has KITE_API_KEY, KITE_API_SECRET, KITE_ACCESS_TOKEN
go run main.go
```

### Frontend Development
```bash
cd frontend
npm install
npm start
```

## Stopping Services

```bash
docker-compose down
```

To remove volumes (deletes all data):
```bash
docker-compose down -v
```

## Notes

- Zerodha access token expires at end of day; run `kite-token` to refresh
- Historical sync on startup fetches 1 year of data (~50 stocks × 1 call each)
- Database data persists in `./docker-volumes/mysql-data` even after container stops

## Troubleshooting

1. **Port already in use**: Change ports in `docker-compose.yml`
2. **Database connection errors**: Wait for MySQL to be healthy before starting other services
3. **No price data**: Ensure KITE_ACCESS_TOKEN is set and valid; run `kite-token` to refresh
4. **Insufficient permission**: Ensure Kite Connect paid subscription (not Personal/free app)
5. **Frontend can't connect to backend**: Check `REACT_APP_API_URL` in frontend environment
